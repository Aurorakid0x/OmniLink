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
	// 获取当前时区偏移量，例如 "+08:00"
	_, offset := now.Zone()
	hours := offset / 3600
	minutes := (offset % 3600) / 60
	zoneStr := fmt.Sprintf("%+02d:%02d", hours, minutes)

	sys := fmt.Sprintf(`你是智能定时任务参数生成器。根据用户输入生成JSON参数，用于创建AI定时任务。

当前时间（参考）：%s (时区: %s)

字段说明：
- action: 固定为 "create"
- trigger_type: 触发类型，可选值：once（一次性）、cron（周期性）、event（事件驱动）
- trigger_value: 触发值
  * once类型：**必须**使用RFC3339格式的绝对时间，且**必须包含时区信息**。
    例如：当前是 10:00，用户说"10分钟后"，你应该计算出 10:10，并输出 "%s" (注意最后的时区标识)。
    ❌ 错误格式：2006-01-02T15:04:05 (丢失时区，会被当做UTC处理)
    ✅ 正确格式：2006-01-02T15:04:05%s
  * cron类型：5段cron表达式，如 "0 8 * * *"（每天8点）
  * event类型：事件key，如 "user_login"
- prompt: **这是任务触发时系统发给AI的指令prompt**，AI收到这个prompt后会执行相应的操作
  * 格式：描述AI需要做什么，让AI自主决定如何完成（调用哪些工具）
  * 示例："用户设置了定时任务，需要在早上8点提醒用户查看今天的日程安排"
  * 示例："用户想要查询好友列表，请调用list_friends工具获取好友信息，然后以友好的方式告知用户"
  * **注意**：不要把用户想要收到的消息内容直接作为prompt，而是告诉AI"用户想要什么"
- agent_id: 可选，执行任务的Agent ID

只输出JSON，不要额外文本。`,
		now.Format(time.RFC3339),
		zoneStr,
		now.Add(10*time.Minute).Format("2006-01-02T15:04:05")+zoneStr,
		zoneStr)
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
