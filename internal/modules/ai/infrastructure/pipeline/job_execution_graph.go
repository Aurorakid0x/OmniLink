package pipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	"OmniLink/internal/modules/ai/application/dto/respond"
	"OmniLink/pkg/zlog"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

type jobExecutionState struct {
	Req            *JobExecutionRequest
	SessionID      string
	AgentID        string
	RetrievedCtx   []respond.CitationEntry
	PromptMsgs     []schema.Message
	Answer         string
	Citations      []respond.CitationEntry
	Tokens         TokenStats
	Err            error
	IterationCount int
	MaxIterations  int
	LastResponse   *schema.Message
	ToolCalls      []string // 记录调用过的工具名称
}

func convertToPointers(msgs []schema.Message) []*schema.Message {
	result := make([]*schema.Message, len(msgs))
	for i := range msgs {
		result[i] = &msgs[i]
	}
	return result
}

func (p *JobExecutionPipeline) loadContextNode(ctx context.Context, req *JobExecutionRequest, _ ...any) (*jobExecutionState, error) {
	st := &jobExecutionState{
		Req:            req,
		SessionID:      strings.TrimSpace(req.SessionID),
		AgentID:        strings.TrimSpace(req.AgentID),
		MaxIterations:  10,
		IterationCount: 0,
	}

	zlog.Info("job execution started",
		zap.String("tenant_user_id", strings.TrimSpace(req.TenantUserID)),
		zap.String("agent_id", st.AgentID),
		zap.String("session_id", st.SessionID),
		zap.Int("prompt_len", len(strings.TrimSpace(req.Prompt))))

	if strings.TrimSpace(req.TenantUserID) == "" {
		st.Err = fmt.Errorf("tenant_user_id is required")
		return st, nil
	}
	if strings.TrimSpace(req.Prompt) == "" {
		st.Err = fmt.Errorf("prompt is required")
		return st, nil
	}
	if strings.TrimSpace(req.SessionID) == "" {
		st.Err = fmt.Errorf("session_id is required")
		return st, nil
	}
	if strings.TrimSpace(req.AgentID) == "" {
		st.Err = fmt.Errorf("agent_id is required")
		return st, nil
	}

	return st, nil
}

func (p *JobExecutionPipeline) retrieveNode(ctx context.Context, st *jobExecutionState, _ ...any) (*jobExecutionState, error) {
	if st == nil || st.Err != nil {
		return st, nil
	}

	topK := st.Req.TopK
	if topK <= 0 {
		topK = 5
	}

	if p.retrievePipe != nil {
		retrieveReq := &RetrieveRequest{
			TenantUserID: st.Req.TenantUserID,
			Question:     st.Req.Prompt,
			TopK:         topK,
			KBType:       "global",
		}
		retrieveRes, err := p.retrievePipe.Retrieve(ctx, retrieveReq)
		if err == nil && retrieveRes != nil && len(retrieveRes.Chunks) > 0 {
			for _, chunk := range retrieveRes.Chunks {
				st.RetrievedCtx = append(st.RetrievedCtx, respond.CitationEntry{
					ChunkID:    fmt.Sprintf("%d", chunk.ChunkID),
					Content:    chunk.Content,
					SourceType: chunk.SourceType,
					SourceKey:  chunk.SourceKey,
					Score:      chunk.Score,
				})
			}
			zlog.Info("job execution rag retrieved", zap.Int("items", len(st.RetrievedCtx)))
		}
	}

	return st, nil
}

func (p *JobExecutionPipeline) buildPromptNode(ctx context.Context, st *jobExecutionState, _ ...any) (*jobExecutionState, error) {
	if st == nil || st.Err != nil {
		return st, nil
	}

	systemPrompt := "你是 OmniLink 的AI助手，正在后台执行用户设置的定时任务。任务**已经触发**，现在是执行时间。\n\n**核心指令**：\n1. **直接执行**任务要求的动作（如发送通知）。\n2. **禁止**再次检查时间（get_current_time）。\n3. **禁止**试图重新创建或管理任务。\n4. 如果需要通知用户，**必须**调用 `push_notification` 工具。\n5. 不要直接输出文本回复。"

	if len(st.RetrievedCtx) > 0 {
		var ctxParts []string
		for i, item := range st.RetrievedCtx {
			if i >= 3 {
				break
			}
			ctxParts = append(ctxParts, item.Content)
		}
		contextStr := strings.Join(ctxParts, "\n\n")
		systemPrompt += fmt.Sprintf("\n\n相关上下文信息：\n%s", contextStr)
		st.Citations = st.RetrievedCtx
	}

	st.PromptMsgs = []schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: st.Req.Prompt},
	}

	zlog.Info("job execution prompt built",
		zap.Int("system_len", len(systemPrompt)),
		zap.Int("prompt_len", len(st.Req.Prompt)),
		zap.Int("citations", len(st.Citations)))

	return st, nil
}

func (p *JobExecutionPipeline) chatModelNode(ctx context.Context, st *jobExecutionState, _ ...any) (*jobExecutionState, error) {
	if st == nil || st.Err != nil {
		return st, nil
	}

	opts := []model.Option{}
	if len(p.tools) > 0 {
		var toolInfos []*schema.ToolInfo
		for _, t := range p.tools {
			info, err := t.Info(ctx)
			if err == nil && info != nil {
				toolInfos = append(toolInfos, info)
			}
		}
		if len(toolInfos) > 0 {
			opts = append(opts, model.WithTools(toolInfos))
		}
	}

	resp, err := p.chatModel.Generate(ctx, convertToPointers(st.PromptMsgs), opts...)
	if err != nil {
		st.Err = err
		return st, nil
	}

	st.LastResponse = resp
	st.Answer = resp.Content

	if resp.ResponseMeta != nil && resp.ResponseMeta.Usage != nil {
		usage := resp.ResponseMeta.Usage
		st.Tokens = TokenStats{
			PromptTokens: usage.PromptTokens,
			AnswerTokens: usage.CompletionTokens,
			TotalTokens:  usage.TotalTokens,
		}
	}

	zlog.Info("job execution llm response",
		zap.Int("iteration", st.IterationCount),
		zap.Int("tool_calls", len(resp.ToolCalls)),
		zap.Int("answer_len", len(resp.Content)))

	if len(resp.ToolCalls) > 0 {
		st.PromptMsgs = append(st.PromptMsgs, *resp)
	}

	return st, nil
}

func (p *JobExecutionPipeline) toolsNode(ctx context.Context, st *jobExecutionState, _ ...any) (*jobExecutionState, error) {
	if st == nil || st.Err != nil {
		return st, nil
	}
	if st.LastResponse == nil || len(st.LastResponse.ToolCalls) == 0 {
		return st, nil
	}

	toolStart := time.Now()

	toolCtx := ctx
	if st.Req != nil {
		tenantUserID := strings.TrimSpace(st.Req.TenantUserID)
		if tenantUserID != "" && toolCtx.Value("tenant_user_id") == nil {
			toolCtx = context.WithValue(toolCtx, "tenant_user_id", tenantUserID)
		}
		agentID := strings.TrimSpace(st.Req.AgentID)
		if agentID != "" && toolCtx.Value("agent_id") == nil {
			toolCtx = context.WithValue(toolCtx, "agent_id", agentID)
		}
		sessionID := strings.TrimSpace(st.Req.SessionID)
		if sessionID != "" && toolCtx.Value("session_id") == nil {
			toolCtx = context.WithValue(toolCtx, "session_id", sessionID)
		}
	}

	var toolMsgs []schema.Message
	for _, tc := range st.LastResponse.ToolCalls {
		toolName := strings.TrimSpace(tc.Function.Name)
		st.ToolCalls = append(st.ToolCalls, toolName) // 记录工具调用

		toolResp := p.invokeTool(toolCtx, tc) // 使用 toolCtx
		toolMsgs = append(toolMsgs, *toolResp)

		zlog.Info("job execution tool executed",
			zap.String("tool_name", toolName),
			zap.String("tool_result", toolResp.Content))
	}

	st.PromptMsgs = append(st.PromptMsgs, toolMsgs...)

	zlog.Info("job execution tools node done",
		zap.String("session_id", st.SessionID),
		zap.Int("tools_executed", len(st.LastResponse.ToolCalls)),
		zap.Int64("tools_ms", time.Since(toolStart).Milliseconds()))

	return st, nil
}

func (p *JobExecutionPipeline) invokeTool(ctx context.Context, tc schema.ToolCall) *schema.Message {
	toolName := strings.TrimSpace(tc.Function.Name)
	toolArgs := strings.TrimSpace(tc.Function.Arguments)

	zlog.Info("job execution invoking tool",
		zap.String("tool_name", toolName),
		zap.String("tool_id", tc.ID))

	for _, t := range p.tools {
		info, _ := t.Info(ctx)
		if info != nil && info.Name == toolName {
			if invokable, ok := t.(tool.InvokableTool); ok {
				result, err := invokable.InvokableRun(ctx, toolArgs)
				if err != nil {
					return &schema.Message{
						Role:       schema.Tool,
						Content:    fmt.Sprintf("Error: %v", err),
						ToolCallID: tc.ID,
					}
				}
				return &schema.Message{
					Role:       schema.Tool,
					Content:    result,
					ToolCallID: tc.ID,
				}
			}
			return &schema.Message{
				Role:       schema.Tool,
				Content:    "Tool not invokable",
				ToolCallID: tc.ID,
			}
		}
	}

	zlog.Info(fmt.Sprintf("job execution tools available: %d", len(p.tools)))
	for _, t := range p.tools {
		info, err := t.Info(ctx)
		if err != nil || info == nil {
			continue
		}
		zlog.Info(fmt.Sprintf("job execution tool: %s", info.Name))
	}

	return &schema.Message{
		Role:       schema.Tool,
		Content:    "Tool not found",
		ToolCallID: tc.ID,
	}
}

func (p *JobExecutionPipeline) persistNode(ctx context.Context, st *jobExecutionState, _ ...any) (*JobExecutionResult, error) {
	if st == nil {
		return &JobExecutionResult{Err: fmt.Errorf("nil state")}, nil
	}
	if st.Err != nil {
		return p.buildFinalResult(st), nil
	}

	if st.LastResponse != nil {
		st.Answer = st.LastResponse.Content
		if st.LastResponse.ResponseMeta != nil {
			usage := st.LastResponse.ResponseMeta.Usage
			if usage != nil {
				st.Tokens = TokenStats{
					PromptTokens: usage.PromptTokens,
					AnswerTokens: usage.CompletionTokens,
					TotalTokens:  usage.TotalTokens,
				}
			}
		}
	}

	// 任务执行流水线不直接保存Assistant消息，而是依赖push_notification工具
	// 这样避免了"系统提示"和"AI回复"的双重消息，也确保了所有通知都通过统一的推送机制

	if err := p.sessionRepo.UpdateSessionUpdatedAt(ctx, st.SessionID); err != nil {
		zlog.Error("failed to update session timestamp", zap.Error(err))
	}

	zlog.Info("job execution completed",
		zap.String("session_id", st.SessionID),
		zap.Int("iterations", st.IterationCount),
		zap.Int("answer_len", len(st.Answer)))

	return p.buildFinalResult(st), nil
}

func (p *JobExecutionPipeline) buildFinalResult(st *jobExecutionState) *JobExecutionResult {
	// Status: 2 (Completed), 3 (Failed)
	status := 2
	summary := "Executed without notification"

	if st.Err != nil {
		status = 3
		summary = fmt.Sprintf("Error: %v", st.Err)
	} else {
		hasPush := false
		for _, toolName := range st.ToolCalls {
			if toolName == "push_notification" {
				hasPush = true
				break
			}
		}
		if hasPush {
			summary = "Notification pushed successfully"
		} else if len(st.ToolCalls) > 0 {
			summary = fmt.Sprintf("Tools executed: %s", strings.Join(st.ToolCalls, ", "))
		}
	}

	return &JobExecutionResult{
		Answer:        st.Answer,
		Citations:     st.Citations,
		TokenStats:    st.Tokens,
		Status:        status,
		ResultSummary: summary,
		Err:           st.Err,
	}
}
