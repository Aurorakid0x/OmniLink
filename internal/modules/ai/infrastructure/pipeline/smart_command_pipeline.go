package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type SmartCommandRequest struct {
	TenantUserID string
	Command      string
	AgentID      string
}

type SmartCommandParams struct {
	Action       string `json:"action"`
	TriggerType  string `json:"trigger_type"`
	TriggerValue string `json:"trigger_value"`
	Prompt       string `json:"prompt"`
	AgentID      string `json:"agent_id"`
}

type SmartCommandResult struct {
	Intent     string
	Params     SmartCommandParams
	ToolName   string
	ToolResult string
	Err        error
}

type SmartCommandPipeline struct {
	chatModel model.BaseChatModel
	tools     []tool.BaseTool
}

func NewSmartCommandPipeline(chatModel model.BaseChatModel, tools []tool.BaseTool) (*SmartCommandPipeline, error) {
	if chatModel == nil {
		return nil, fmt.Errorf("chat model is nil")
	}
	return &SmartCommandPipeline{
		chatModel: chatModel,
		tools:     tools,
	}, nil
}

func (p *SmartCommandPipeline) SetTools(tools []tool.BaseTool) {
	p.tools = tools
}

func (p *SmartCommandPipeline) Execute(ctx context.Context, req *SmartCommandRequest) (*SmartCommandResult, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}
	command := strings.TrimSpace(req.Command)
	if command == "" {
		return nil, fmt.Errorf("command is required")
	}
	if strings.TrimSpace(req.TenantUserID) == "" {
		return nil, fmt.Errorf("tenant_user_id is required")
	}

	intentRes, err := p.recognizeIntent(ctx, command)
	if err != nil {
		return &SmartCommandResult{Err: err}, nil
	}
	if intentRes.Intent != "create_job" {
		return &SmartCommandResult{Intent: intentRes.Intent, Err: errors.New("not a scheduling command")}, nil
	}

	params, err := p.extractParams(ctx, command)
	if err != nil {
		return &SmartCommandResult{Intent: intentRes.Intent, Err: err}, nil
	}
	if strings.TrimSpace(params.Action) == "" {
		params.Action = "create"
	}
	if strings.TrimSpace(params.AgentID) == "" {
		params.AgentID = strings.TrimSpace(req.AgentID)
	}
	if strings.TrimSpace(params.TriggerType) == "" || strings.TrimSpace(params.TriggerValue) == "" || strings.TrimSpace(params.Prompt) == "" {
		return &SmartCommandResult{Intent: intentRes.Intent, Err: errors.New("invalid command parameters")}, nil
	}

	toolName, toolResult, err := p.callManageJob(ctx, params)
	if err != nil {
		return &SmartCommandResult{Intent: intentRes.Intent, Params: params, Err: err}, nil
	}

	return &SmartCommandResult{
		Intent:     intentRes.Intent,
		Params:     params,
		ToolName:   toolName,
		ToolResult: toolResult,
	}, nil
}

type intentResult struct {
	Intent      string `json:"intent"`
	TriggerType string `json:"trigger_type"`
}

func (p *SmartCommandPipeline) recognizeIntent(ctx context.Context, command string) (*intentResult, error) {
	sys := "你是智能指令意图识别器，只输出JSON：{\"intent\":\"create_job\"|\"other\",\"trigger_type\":\"once|cron|event|\"}。当输入表达提醒、定时、周期、登录或事件触发任务时为 create_job。"
	msgs := []*schema.Message{
		{Role: schema.System, Content: sys},
		{Role: schema.User, Content: command},
	}
	resp, err := p.chatModel.Generate(ctx, msgs)
	if err != nil {
		return nil, err
	}
	var out intentResult
	if err := parseJSONFromContent(resp.Content, &out); err != nil {
		return nil, err
	}
	out.Intent = strings.TrimSpace(out.Intent)
	out.TriggerType = strings.TrimSpace(out.TriggerType)
	if out.Intent == "" {
		return nil, errors.New("intent parse failed")
	}
	return &out, nil
}

func (p *SmartCommandPipeline) extractParams(ctx context.Context, command string) (SmartCommandParams, error) {
	now := time.Now()
	sys := fmt.Sprintf("你是智能指令参数生成器。根据用户输入生成JSON，字段：action(固定为create)、trigger_type(once|cron|event)、trigger_value(once为RFC3339时间，cron为5段表达式，event为事件key如user_login/new_friend_apply/group_mention)、prompt(任务执行时发送给Agent的指令，必须包含对push_notification的调用并给出明确内容)、agent_id(可空)。只输出JSON，不要额外文本。当前时间：%s。", now.Format(time.RFC3339))
	msgs := []*schema.Message{
		{Role: schema.System, Content: sys},
		{Role: schema.User, Content: command},
	}
	resp, err := p.chatModel.Generate(ctx, msgs)
	if err != nil {
		return SmartCommandParams{}, err
	}
	var out SmartCommandParams
	if err := parseJSONFromContent(resp.Content, &out); err != nil {
		return SmartCommandParams{}, err
	}
	out.Action = strings.TrimSpace(out.Action)
	out.TriggerType = strings.TrimSpace(out.TriggerType)
	out.TriggerValue = strings.TrimSpace(out.TriggerValue)
	out.Prompt = strings.TrimSpace(out.Prompt)
	out.AgentID = strings.TrimSpace(out.AgentID)
	return out, nil
}

func (p *SmartCommandPipeline) callManageJob(ctx context.Context, params SmartCommandParams) (string, string, error) {
	if len(p.tools) == 0 {
		return "", "", errors.New("tools not initialized")
	}
	tools, toolInfos := filterTools(ctx, p.tools, "manage_ai_job")
	if len(tools) == 0 {
		return "", "", errors.New("manage_ai_job tool not available")
	}

	args := map[string]interface{}{
		"action":        params.Action,
		"trigger_type":  params.TriggerType,
		"trigger_value": params.TriggerValue,
		"prompt":        params.Prompt,
	}
	if params.AgentID != "" {
		args["agent_id"] = params.AgentID
	}
	argsJSON, _ := json.Marshal(args)

	sys := "你是工具调度器，只能调用 manage_ai_job。必须使用提供的参数进行调用，不要修改、不返回额外文本。"
	msgs := []*schema.Message{
		{Role: schema.System, Content: sys},
		{Role: schema.User, Content: string(argsJSON)},
	}
	resp, err := p.chatModel.Generate(ctx, msgs, model.WithTools(toolInfos))
	if err != nil {
		return "", "", err
	}
	if len(resp.ToolCalls) == 0 {
		return "", "", errors.New("no tool call generated")
	}
	toolCall := resp.ToolCalls[0]
	toolName := strings.TrimSpace(toolCall.Function.Name)
	if toolName == "" {
		toolName = "manage_ai_job"
	}
	toolArgs := strings.TrimSpace(toolCall.Function.Arguments)
	if toolArgs == "" {
		toolArgs = string(argsJSON)
	}
	toolResp, err := invokeTool(ctx, tools, toolName, toolArgs)
	if err != nil {
		return toolName, "", err
	}
	return toolName, toolResp, nil
}

func filterTools(ctx context.Context, tools []tool.BaseTool, name string) ([]tool.BaseTool, []*schema.ToolInfo) {
	var picked []tool.BaseTool
	var infos []*schema.ToolInfo
	for _, t := range tools {
		info, err := t.Info(ctx)
		if err != nil || info == nil {
			continue
		}
		if info.Name == name {
			picked = append(picked, t)
			infos = append(infos, info)
		}
	}
	return picked, infos
}

func invokeTool(ctx context.Context, tools []tool.BaseTool, name string, args string) (string, error) {
	for _, t := range tools {
		info, _ := t.Info(ctx)
		if info != nil && info.Name == name {
			if invokable, ok := t.(tool.InvokableTool); ok {
				return invokable.InvokableRun(ctx, args)
			}
			return "", errors.New("tool is not invokable")
		}
	}
	return "", errors.New("tool not found")
}

func parseJSONFromContent(content string, out interface{}) error {
	raw := extractJSONObject(content)
	if raw == "" {
		return errors.New("json not found")
	}
	return json.Unmarshal([]byte(raw), out)
}

func extractJSONObject(content string) string {
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	return content[start : end+1]
}
