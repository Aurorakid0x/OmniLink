package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"OmniLink/internal/modules/ai/application/dto/respond"
	"OmniLink/internal/modules/ai/domain/assistant"
	"OmniLink/internal/modules/ai/domain/rag"
	"OmniLink/pkg/util"
	"OmniLink/pkg/zlog"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// assistantState Graph内部状态（在节点间传递）
type assistantState struct {
	Req            *AssistantRequest
	SessionID      string
	IsNewSession   bool
	Messages       []*assistant.AIAssistantMessage
	RetrievedCtx   []respond.CitationEntry
	PromptMsgs     []schema.Message
	Answer         string
	Citations      []respond.CitationEntry
	Tokens         TokenStats
	QueryID        string
	Start          time.Time
	EmbeddingMs    int64
	SearchMs       int64
	PostProcessMs  int64
	LLMMs          int64
	Tools          []*schema.ToolInfo
	Err            error
	StreamEmitter  StreamEmitter
	IterationCount int // 当前循环次数
	MaxIterations  int
	LastResponse   *schema.Message
	TaskSuggestion *TaskSuggestion
}

const defaultPersonaPrompt = "你是 OmniLink 的全局 AI 个人助手，回答必须基于用户权限内的聊天/联系人/群组信息。"

// Node 1: LoadMemory - 加载历史消息
func (p *AssistantPipeline) loadMemoryNode(ctx context.Context, req *AssistantRequest, _ ...any) (*assistantState, error) {
	st := &assistantState{
		Req:            req,
		Start:          time.Now(),
		QueryID:        fmt.Sprintf("q_%s_%d", util.GenerateID("Q"), time.Now().UnixNano()),
		MaxIterations:  10, // 默认最多10轮ReAct循环
		IterationCount: 0,
	}
	zlog.Info("assistant load memory start",
		zap.String("query_id", st.QueryID),
		zap.String("tenant_user_id", strings.TrimSpace(req.TenantUserID)),
		zap.String("session_id", strings.TrimSpace(req.SessionID)),
		zap.String("agent_id", strings.TrimSpace(req.AgentID)),
		zap.Int("question_len", len(strings.TrimSpace(req.Question))))

	// 1. 校验必填参数
	if strings.TrimSpace(req.TenantUserID) == "" {
		st.Err = fmt.Errorf("tenant_user_id is required")
		return st, nil
	}
	if strings.TrimSpace(req.Question) == "" {
		st.Err = fmt.Errorf("question is required")
		return st, nil
	}

	// 2. 处理会话
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		// 创建新会话
		now := time.Now()
		newSession := &assistant.AIAssistantSession{
			SessionId:    util.GenerateID("AS"), // AS = Assistant Session
			TenantUserId: req.TenantUserID,
			Title:        truncateTitle(req.Question),
			Status:       assistant.SessionStatusActive,
			AgentId:      strings.TrimSpace(req.AgentID),
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := p.sessionRepo.CreateSession(ctx, newSession); err != nil {
			st.Err = err
			return st, nil
		}
		st.SessionID = newSession.SessionId
		st.IsNewSession = true
		st.Messages = []*assistant.AIAssistantMessage{} // 新会话无历史
		zlog.Info("assistant session created",
			zap.String("query_id", st.QueryID),
			zap.String("session_id", st.SessionID),
			zap.String("tenant_user_id", req.TenantUserID),
			zap.String("agent_id", newSession.AgentId))
	} else {
		// 加载现有会话
		sess, err := p.sessionRepo.GetSessionByID(ctx, sessionID, req.TenantUserID)
		if err != nil {
			st.Err = err
			return st, nil
		}
		if sess == nil {
			st.Err = fmt.Errorf("session not found or access denied")
			return st, nil
		}
		st.SessionID = sessionID
		st.IsNewSession = false
		if strings.TrimSpace(req.AgentID) == "" {
			req.AgentID = strings.TrimSpace(sess.AgentId)
		}

		// 加载最近6轮对话（12条消息）
		messages, err := p.messageRepo.ListRecentMessages(ctx, sessionID, 12)
		if err != nil {
			st.Err = err
			return st, nil
		}
		st.Messages = messages
		zlog.Info("assistant session loaded",
			zap.String("query_id", st.QueryID),
			zap.String("session_id", st.SessionID),
			zap.String("tenant_user_id", req.TenantUserID),
			zap.String("agent_id", req.AgentID),
			zap.Int("history_count", len(messages)))
	}

	zlog.Info("assistant load memory done",
		zap.String("session_id", st.SessionID),
		zap.Bool("is_new", st.IsNewSession),
		zap.Int("history_count", len(st.Messages)))

	return st, nil
}

// Node 2: Retrieve - RAG召回
func (p *AssistantPipeline) retrieveNode(ctx context.Context, st *assistantState, _ ...any) (*assistantState, error) {
	if st == nil || st.Err != nil {
		return st, nil
	}

	req := st.Req
	scope := normalizeScope(req.Scope)
	topK := normalizeTopK(req.TopK)
	zlog.Info("assistant retrieve start",
		zap.String("query_id", st.QueryID),
		zap.String("tenant_user_id", req.TenantUserID),
		zap.String("scope", scope),
		zap.Int("top_k", topK),
		zap.Int("source_keys", len(req.SourceKeys)))

	// 获取知识库ID
	kbID, err := p.ensureKnowledgeBase(ctx, req.TenantUserID, scope)
	if err != nil {
		st.Err = err
		return st, nil
	}

	// 构建检索请求
	retrieveReq := &RetrieveRequest{
		TenantUserID: req.TenantUserID,
		Question:     req.Question,
		TopK:         topK,
		KBType:       scope,
	}

	// 可选：限制source_keys
	if len(req.SourceKeys) > 0 {
		retrieveReq.SourceKeys = req.SourceKeys
	}

	// 执行检索
	retrieveStart := time.Now()
	result, err := p.retrievePipe.Retrieve(ctx, retrieveReq)
	if err != nil {
		st.Err = err
		return st, nil
	}

	st.EmbeddingMs = result.EmbeddingMs
	st.SearchMs = result.SearchMs
	st.PostProcessMs = result.PostProcessMs

	// 转换为Citations
	citations := make([]respond.CitationEntry, 0, len(result.Chunks))
	for _, chunk := range result.Chunks {
		citations = append(citations, respond.CitationEntry{
			ChunkID:    fmt.Sprintf("%d", chunk.ChunkID),
			SourceType: chunk.SourceType,
			SourceKey:  chunk.SourceKey,
			Score:      chunk.Score,
			Content:    truncateContent(chunk.Content, 200),
		})
	}
	st.RetrievedCtx = citations
	st.Citations = citations

	zlog.Info("assistant retrieve done",
		zap.String("query_id", st.QueryID),
		zap.Int("kb_id", int(kbID)),
		zap.Int("chunks", len(citations)),
		zap.Int64("retrieve_ms", time.Since(retrieveStart).Milliseconds()))

	return st, nil
}

// Node 3: BuildPrompt - 构建Prompt
func (p *AssistantPipeline) buildPromptNode(ctx context.Context, st *assistantState, _ ...any) (*assistantState, error) {
	if st == nil || st.Err != nil {
		return st, nil
	}

	promptMsgs := make([]schema.Message, 0, 2+len(st.Messages)+2)

	userName := ""
	if p.userRepo != nil {
		briefs, err := p.userRepo.GetUserBriefByUUIDs([]string{st.Req.TenantUserID})
		if err == nil && len(briefs) > 0 {
			name := strings.TrimSpace(briefs[0].Nickname)
			if name == "" {
				name = strings.TrimSpace(briefs[0].Username)
			}
			userName = name
		}
	}
	if userName == "" {
		userName = st.Req.TenantUserID
	}
	promptMsgs = append(promptMsgs, schema.Message{
		Role:    schema.System,
		Content: fmt.Sprintf("### 用户上下文\n用户ID: %s\n用户名称: %s\n你可以使用以上已提供的用户信息来回答与该用户相关的问题，但不得臆造未提供的信息。", st.Req.TenantUserID, userName),
	})

	personaPrompt := defaultPersonaPrompt
	agentID := strings.TrimSpace(st.Req.AgentID)
	if p.agentRepo != nil && agentID != "" {
		ag, err := p.agentRepo.GetAgentByID(ctx, agentID, st.Req.TenantUserID)
		if err != nil {
			st.Err = err
			return st, nil
		}
		if ag != nil {
			systemPrompt := strings.TrimSpace(ag.SystemPrompt)
			persona := strings.TrimSpace(ag.PersonaPrompt)
			desc := strings.TrimSpace(ag.Description)
			if systemPrompt != "" {
				personaPrompt = fmt.Sprintf("%s\n[System Prompt]\n%s", personaPrompt, systemPrompt)
			}
			if persona != "" || desc != "" {
				personaPrompt = fmt.Sprintf("%s\n[User Persona]\n%s", personaPrompt, strings.TrimSpace(strings.Join([]string{persona, desc}, "\n")))
			}
		}
	}
	// 在对话提示里要求模型输出结构化任务建议，供后置路由解析
	personaPrompt = fmt.Sprintf("%s\n请在最终回答末尾追加%s{\"should_create\":true|false,\"command\":\"...\"}%s。若判断用户意图是创建提醒、定时或事件任务，则should_create为true，否则为false。command填写用于创建任务的自然语言指令。", personaPrompt, taskSuggestionStart, taskSuggestionEnd)
	promptMsgs = append(promptMsgs, schema.Message{
		Role:    schema.System,
		Content: personaPrompt,
	})
	zlog.Info("assistant build prompt start",
		zap.String("query_id", st.QueryID),
		zap.String("tenant_user_id", st.Req.TenantUserID),
		zap.String("agent_id", agentID),
		zap.Int("history_count", len(st.Messages)),
		zap.Int("retrieved_chunks", len(st.RetrievedCtx)))

	// 2. 历史消息（最近N轮）
	for _, msg := range st.Messages {
		role := schema.User
		switch msg.Role {
		case "assistant":
			role = schema.Assistant
		case "system":
			role = schema.System
		}
		promptMsgs = append(promptMsgs, schema.Message{
			Role:    role,
			Content: msg.Content,
		})
	}

	// 3. Retrieved Context（如果有）
	if len(st.RetrievedCtx) > 0 {
		contextStr := buildContextString(st.RetrievedCtx)
		promptMsgs = append(promptMsgs, schema.Message{
			Role:    schema.System,
			Content: fmt.Sprintf("以下是检索到的相关上下文信息：\n%s", contextStr),
		})
	}

	// 4. 当前用户问题
	promptMsgs = append(promptMsgs, schema.Message{
		Role:    schema.User,
		Content: st.Req.Question,
	})

	st.PromptMsgs = promptMsgs

	// 获取可用工具
	var toolInfos []*schema.ToolInfo
	if len(p.tools) > 0 {
		toolInfos = make([]*schema.ToolInfo, 0, len(p.tools))
		for _, t := range p.tools {
			info, err := t.Info(ctx)
			if err != nil {
				zlog.Warn("failed to get tool info", zap.Error(err))
				continue
			}
			toolInfos = append(toolInfos, info)
		}
		st.Tools = toolInfos
	}

	promptJSON := ""
	if b, err := json.Marshal(promptMsgs); err == nil {
		promptJSON = string(b)
	} else {
		promptJSON = fmt.Sprintf("marshal_prompt_error:%v", err)
	}
	toolsJSON := ""
	if b, err := json.Marshal(st.Tools); err == nil {
		toolsJSON = string(b)
	} else {
		toolsJSON = fmt.Sprintf("marshal_tools_error:%v", err)
	}

	zlog.Info("assistant build prompt done",
		zap.String("query_id", st.QueryID),
		zap.Int("prompt_msgs", len(promptMsgs)),
		zap.Int("history_msgs", len(st.Messages)),
		zap.Int("retrieved_chunks", len(st.RetrievedCtx)),
		zap.String("prompt", promptJSON),
		zap.String("tools", toolsJSON))

	return st, nil
}

// Node 4: ChatModel - 调用LLM（ReAct模式：只调用LLM，不执行工具）
func (p *AssistantPipeline) chatModelNode(ctx context.Context, st *assistantState, _ ...any) (*assistantState, error) {
	if st == nil || st.Err != nil {
		return st, nil
	}

	llmStart := time.Now()
	zlog.Info("assistant chat model start",
		zap.String("query_id", st.QueryID),
		zap.Int("iteration", st.IterationCount+1),
		zap.Int("prompt_msgs", len(st.PromptMsgs)),
		zap.Int("tools", len(st.Tools)))

	promptMsgs := make([]*schema.Message, len(st.PromptMsgs))
	for i := range st.PromptMsgs {
		promptMsgs[i] = &st.PromptMsgs[i]
	}

	// 调用 LLM（传入工具定义）
	var resp *schema.Message
	var err error
	if len(st.Tools) > 0 {
		resp, err = p.chatModel.Generate(ctx, promptMsgs, model.WithTools(st.Tools))
	} else {
		resp, err = p.chatModel.Generate(ctx, promptMsgs)
	}
	if err != nil {
		st.Err = err
		return st, nil
	}

	if resp != nil && resp.Content != "" {
		// 解析并剥离结构化建议，避免污染用户可见回答
		sugg, cleaned := parseTaskSuggestion(resp.Content)
		if sugg != nil {
			st.TaskSuggestion = sugg
		}
		resp.Content = cleaned
	}

	st.LastResponse = resp
	st.PromptMsgs = append(st.PromptMsgs, *resp)
	if resp != nil && resp.ResponseMeta != nil && resp.ResponseMeta.Usage != nil {
		usage := resp.ResponseMeta.Usage
		zlog.Info("assistant chat model usage",
			zap.String("query_id", st.QueryID),
			zap.Int("prompt_tokens", usage.PromptTokens),
			zap.Int("answer_tokens", usage.CompletionTokens),
			zap.Int("total_tokens", usage.TotalTokens))
	}

	// 累加LLM耗时
	st.LLMMs += time.Since(llmStart).Milliseconds()

	// 递增迭代计数
	st.IterationCount++

	zlog.Info("assistant chat model iteration",
		zap.String("query_id", st.QueryID),
		zap.Int("iteration", st.IterationCount),
		zap.Int("tool_calls", len(resp.ToolCalls)),
		zap.Int64("llm_ms", time.Since(llmStart).Milliseconds()))

	return st, nil
}

// Node 5: Tools - 执行工具调用（ReAct模式新增节点）
func (p *AssistantPipeline) toolsNode(ctx context.Context, st *assistantState, _ ...any) (*assistantState, error) {
	if st == nil || st.Err != nil {
		return st, nil
	}

	// 从最后一条消息提取 tool calls
	if st.LastResponse == nil || len(st.LastResponse.ToolCalls) == 0 {
		// 没有工具调用，直接返回
		return st, nil
	}

	toolStart := time.Now()
	zlog.Info("assistant tools node start",
		zap.String("query_id", st.QueryID),
		zap.Int("tool_calls", len(st.LastResponse.ToolCalls)))

	// 执行所有工具调用
	for _, toolCall := range st.LastResponse.ToolCalls {
		toolName := toolCall.Function.Name
		toolArgs := toolCall.Function.Arguments

		var toolResp string
		var found bool
		var runErr error

		if st.StreamEmitter != nil {
			st.StreamEmitter("tool_call", map[string]string{"tool_name": toolName})
		}
		zlog.Info("assistant tool call start",
			zap.String("query_id", st.QueryID),
			zap.String("tool_name", toolName),
			zap.Int("args_len", len(toolArgs)))

		for _, t := range p.tools {
			info, _ := t.Info(ctx)
			if info != nil && info.Name == toolName {
				found = true
				// 执行工具
				if invokable, ok := t.(tool.InvokableTool); ok {
					res, err := invokable.InvokableRun(ctx, toolArgs)
					if err != nil {
						runErr = err
						toolResp = fmt.Sprintf("Tool execution error: %v", err)
					} else {
						toolResp = res
					}
				} else {
					runErr = fmt.Errorf("tool is not invokable")
					toolResp = "Tool is not invokable"
				}
				break
			}
		}

		if !found {
			runErr = fmt.Errorf("tool not found")
			toolResp = fmt.Sprintf("Tool '%s' not found", toolName)
		}
		if runErr != nil {
			zlog.Info("assistant tool call error",
				zap.String("query_id", st.QueryID),
				zap.String("tool_name", toolName),
				zap.Error(runErr))
		}

		if st.StreamEmitter != nil {
			status := "success"
			if runErr != nil {
				status = "error"
			}
			st.StreamEmitter("tool_result", map[string]string{"tool_name": toolName, "status": status})
		}

		// 获取tool call ID
		toolCallID := toolCall.ID
		if toolCallID == "" {
			toolCallID = toolName
		}

		// 将工具响应添加到消息历史
		st.PromptMsgs = append(st.PromptMsgs, *schema.ToolMessage(toolResp, toolCallID, schema.WithToolName(toolName)))

		zlog.Info("assistant tool executed",
			zap.String("query_id", st.QueryID),
			zap.String("tool_name", toolName),
			zap.Int("response_len", len(toolResp)))
	}

	zlog.Info("assistant tools node done",
		zap.String("query_id", st.QueryID),
		zap.Int("tools_executed", len(st.LastResponse.ToolCalls)),
		zap.Int64("tools_ms", time.Since(toolStart).Milliseconds()))

	return st, nil
}

// Node 6: Persist - 持久化消息（ReAct模式：从LastResponse提取最终答案）
func (p *AssistantPipeline) persistNode(ctx context.Context, st *assistantState, _ ...any) (*AssistantResult, error) {
	if st == nil {
		return &AssistantResult{Err: fmt.Errorf("nil state")}, nil
	}
	if st.Err != nil {
		return p.buildFinalResult(st), nil
	}

	// 从LastResponse提取最终答案和token统计
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
	if st.Answer != "" {
		// 再次兜底解析，避免流式/非流式路径遗漏结构化建议
		sugg, cleaned := parseTaskSuggestion(st.Answer)
		if sugg != nil {
			st.TaskSuggestion = sugg
		}
		st.Answer = cleaned
		if st.LastResponse != nil {
			st.LastResponse.Content = cleaned
		}
	}
	zlog.Info("assistant persist start",
		zap.String("query_id", st.QueryID),
		zap.String("session_id", st.SessionID),
		zap.Int("answer_len", len(st.Answer)),
		zap.Int("citations", len(st.Citations)))

	now := time.Now()

	// 1. 保存user消息
	userMsg := &assistant.AIAssistantMessage{
		SessionId:     st.SessionID,
		Role:          "user",
		Content:       st.Req.Question,
		CitationsJson: "[]",
		TokensJson:    "{}",
		CreatedAt:     now,
	}
	if err := p.messageRepo.SaveMessage(ctx, userMsg); err != nil {
		zlog.Error("failed to save user message", zap.Error(err))
		// 不阻断流程
	}

	// 2. 保存assistant消息
	citationsJSON := "{}"
	if len(st.Citations) > 0 {
		if b, err := json.Marshal(st.Citations); err == nil {
			citationsJSON = string(b)
		}
	}

	tokensJSON := "{}"
	if st.Tokens.TotalTokens > 0 {
		if b, err := json.Marshal(st.Tokens); err == nil {
			tokensJSON = string(b)
		}
	}

	assistantMsg := &assistant.AIAssistantMessage{
		SessionId:     st.SessionID,
		Role:          "assistant",
		Content:       st.Answer,
		CitationsJson: citationsJSON,
		TokensJson:    tokensJSON,
		CreatedAt:     now,
	}
	if err := p.messageRepo.SaveMessage(ctx, assistantMsg); err != nil {
		zlog.Error("failed to save assistant message", zap.Error(err))
	}

	// 3. 更新session的updated_at
	if err := p.sessionRepo.UpdateSessionUpdatedAt(ctx, st.SessionID); err != nil {
		zlog.Error("failed to update session timestamp", zap.Error(err))
	}

	if st.TaskSuggestion != nil && st.TaskSuggestion.ShouldCreate && p.smartCommandPipe != nil {
		// 后置路由：先正常对话，必要时再调用智能指令流水线创建任务
		command := strings.TrimSpace(st.TaskSuggestion.Command)
		if command == "" {
			command = strings.TrimSpace(st.Req.Question)
		}
		agentID := strings.TrimSpace(st.Req.AgentID)
		if agentID == "" && p.agentRepo != nil {
			if ag, err := p.agentRepo.GetSystemGlobalAgent(ctx, st.Req.TenantUserID); err == nil && ag != nil {
				agentID = strings.TrimSpace(ag.AgentId)
			}
		}
		pipeReq := &SmartCommandRequest{
			TenantUserID: st.Req.TenantUserID,
			Command:      command,
			AgentID:      agentID,
		}
		pipeCtx := context.WithValue(ctx, "tenant_user_id", st.Req.TenantUserID)
		if agentID != "" {
			pipeCtx = context.WithValue(pipeCtx, "agent_id", agentID)
		}
		_, _ = p.smartCommandPipe.Execute(pipeCtx, pipeReq)
		zlog.Info("assistant post route executed",
			zap.String("query_id", st.QueryID),
			zap.String("tenant_user_id", st.Req.TenantUserID),
			zap.String("agent_id", agentID),
			zap.Int("command_len", len(command)))
	} else if st.TaskSuggestion == nil {
		zlog.Info("assistant post route skipped",
			zap.String("query_id", st.QueryID),
			zap.String("reason", "no_task_suggestion"))
	} else if !st.TaskSuggestion.ShouldCreate {
		zlog.Info("assistant post route skipped",
			zap.String("query_id", st.QueryID),
			zap.String("reason", "should_create_false"))
	} else if p.smartCommandPipe == nil {
		zlog.Info("assistant post route skipped",
			zap.String("query_id", st.QueryID),
			zap.String("reason", "smart_command_pipeline_nil"))
	}

	zlog.Info("assistant persist done",
		zap.String("session_id", st.SessionID),
		zap.String("query_id", st.QueryID),
		zap.Int("total_iterations", st.IterationCount))

	return p.buildFinalResult(st), nil
}

// 辅助函数

func (p *AssistantPipeline) ensureKnowledgeBase(ctx context.Context, tenantUserID, kbType string) (int64, error) {
	now := time.Now()
	kb := &rag.AIKnowledgeBase{
		OwnerType: "user",
		OwnerId:   tenantUserID,
		KBType:    kbType,
		Name:      kbType,
		Status:    rag.CommonStatusEnabled,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return p.ragRepo.EnsureKnowledgeBase(ctx, kb)
}

func buildContextString(citations []respond.CitationEntry) string {
	var sb strings.Builder
	for i, c := range citations {
		sb.WriteString(fmt.Sprintf("[chunk:%s] %s (来源: %s/%s, 得分: %.3f)\n",
			c.ChunkID, c.Content, c.SourceType, c.SourceKey, c.Score))
		if i >= 4 { // 最多展示5个
			break
		}
	}
	return sb.String()
}

// TaskSuggestion LLM 返回的结构化任务建议
type TaskSuggestion struct {
	ShouldCreate bool   `json:"should_create"`
	Command      string `json:"command"`
}

const (
	// 建议标记用于与正文隔离，便于解析与清理
	taskSuggestionStart = "<task_suggestion>"
	taskSuggestionEnd   = "</task_suggestion>"
)

// parseTaskSuggestion 从回答中解析建议并返回清理后的正文
func parseTaskSuggestion(content string) (*TaskSuggestion, string) {
	start := strings.Index(content, taskSuggestionStart)
	if start < 0 {
		return nil, strings.TrimSpace(content)
	}
	end := strings.Index(content[start+len(taskSuggestionStart):], taskSuggestionEnd)
	if end < 0 {
		return nil, strings.TrimSpace(content[:start])
	}
	end = start + len(taskSuggestionStart) + end
	raw := strings.TrimSpace(content[start+len(taskSuggestionStart) : end])
	var sugg TaskSuggestion
	if err := json.Unmarshal([]byte(raw), &sugg); err != nil {
		return nil, strings.TrimSpace(content[:start] + content[end+len(taskSuggestionEnd):])
	}
	cleaned := strings.TrimSpace(content[:start] + content[end+len(taskSuggestionEnd):])
	return &sugg, cleaned
}

func parseToolCall(call schema.ToolCall) (string, map[string]interface{}, error) {
	payload, err := json.Marshal(call)
	if err != nil {
		return "", nil, err
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return "", nil, err
	}

	name := ""
	if v, ok := raw["name"].(string); ok {
		name = v
	}

	var fn map[string]interface{}
	if v, ok := raw["function"].(map[string]interface{}); ok {
		fn = v
	}
	if name == "" && fn != nil {
		if v, ok := fn["name"].(string); ok {
			name = v
		}
	}

	args := make(map[string]interface{})
	var argsVal interface{}
	if v, ok := raw["arguments"]; ok {
		argsVal = v
	} else if fn != nil {
		if v, ok := fn["arguments"]; ok {
			argsVal = v
		}
	}

	switch v := argsVal.(type) {
	case string:
		if v != "" {
			_ = json.Unmarshal([]byte(v), &args)
		}
	case map[string]interface{}:
		args = v
	}

	return name, args, nil
}

func truncateTitle(question string) string {
	runes := []rune(question)
	if len(runes) > 30 {
		return string(runes[:30]) + "..."
	}
	return question
}

func truncateContent(content string, maxLen int) string {
	runes := []rune(content)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "..."
	}
	return content
}
