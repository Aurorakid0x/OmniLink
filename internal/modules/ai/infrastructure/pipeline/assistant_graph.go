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

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// assistantState Graph内部状态（在节点间传递）
type assistantState struct {
	Req           *AssistantRequest
	SessionID     string
	IsNewSession  bool
	Messages      []*assistant.AIAssistantMessage
	RetrievedCtx  []respond.CitationEntry
	PromptMsgs    []schema.Message
	Answer        string
	Citations     []respond.CitationEntry
	Tokens        TokenStats
	QueryID       string
	Start         time.Time
	EmbeddingMs   int64
	SearchMs      int64
	PostProcessMs int64
	LLMMs         int64
	Err           error
}

const defaultPersonaPrompt = "你是 OmniLink 的全局 AI 个人助手，回答必须基于用户权限内的聊天/联系人/群组信息。"

// Node 1: LoadMemory - 加载历史消息
func (p *AssistantPipeline) loadMemoryNode(ctx context.Context, req *AssistantRequest, _ ...any) (*assistantState, error) {
	st := &assistantState{
		Req:     req,
		Start:   time.Now(),
		QueryID: fmt.Sprintf("q_%s_%d", util.GenerateID("Q"), time.Now().UnixNano()),
	}

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

	promptMsgs := make([]schema.Message, 0, 1+len(st.Messages)+2)

	personaPrompt := defaultPersonaPrompt
	agentID := strings.TrimSpace(st.Req.AgentID)
	if p.agentRepo != nil && agentID != "" {
		ag, err := p.agentRepo.GetAgentByID(ctx, agentID, st.Req.TenantUserID)
		if err != nil {
			st.Err = err
			return st, nil
		}
		if ag != nil {
			persona := strings.TrimSpace(ag.PersonaPrompt)
			desc := strings.TrimSpace(ag.Description)
			if persona != "" && desc != "" {
				personaPrompt = fmt.Sprintf("%s\n%s\n%s", personaPrompt, persona, desc)
			} else if persona != "" {
				personaPrompt = fmt.Sprintf("%s\n%s", personaPrompt, persona)
			} else if desc != "" {
				personaPrompt = fmt.Sprintf("%s\n%s", personaPrompt, desc)
			}
		}
	}
	promptMsgs = append(promptMsgs, schema.Message{
		Role:    schema.System,
		Content: personaPrompt,
	})

	// 2. 历史消息（最近N轮）
	for _, msg := range st.Messages {
		role := schema.User
		if msg.Role == "assistant" {
			role = schema.Assistant
		} else if msg.Role == "system" {
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
			Content: fmt.Sprintf("以下是检索到的相关上下文信息（请基于这些信息回答）：\n%s", contextStr),
		})
	}

	// 4. 当前用户问题
	promptMsgs = append(promptMsgs, schema.Message{
		Role:    schema.User,
		Content: st.Req.Question,
	})

	st.PromptMsgs = promptMsgs

	zlog.Info("assistant build prompt done",
		zap.String("query_id", st.QueryID),
		zap.Int("prompt_msgs", len(promptMsgs)),
		zap.Int("history_msgs", len(st.Messages)),
		zap.Int("retrieved_chunks", len(st.RetrievedCtx)))

	return st, nil
}

// Node 4: ChatModel - 调用LLM（非流式）
func (p *AssistantPipeline) chatModelNode(ctx context.Context, st *assistantState, _ ...any) (*assistantState, error) {
	if st == nil || st.Err != nil {
		return st, nil
	}

	llmStart := time.Now()

	// 转换为指针数组
	promptMsgs := make([]*schema.Message, len(st.PromptMsgs))
	for i := range st.PromptMsgs {
		promptMsgs[i] = &st.PromptMsgs[i]
	}

	// 调用ChatModel
	resp, err := p.chatModel.Generate(ctx, promptMsgs)
	if err != nil {
		st.Err = err
		return st, nil
	}

	st.Answer = resp.Content
	st.LLMMs = time.Since(llmStart).Milliseconds()

	// Token统计（如果有）
	if resp.ResponseMeta != nil {
		usage := resp.ResponseMeta.Usage
		if usage != nil {
			st.Tokens = TokenStats{
				PromptTokens: usage.PromptTokens,
				AnswerTokens: usage.CompletionTokens,
				TotalTokens:  usage.TotalTokens,
			}
		}
	}

	zlog.Info("assistant chat model done",
		zap.String("query_id", st.QueryID),
		zap.Int("answer_len", len(st.Answer)),
		zap.Int64("llm_ms", st.LLMMs),
		zap.Int("tokens", st.Tokens.TotalTokens))

	return st, nil
}

// Node 5: Persist - 持久化消息
func (p *AssistantPipeline) persistNode(ctx context.Context, st *assistantState, _ ...any) (*AssistantResult, error) {
	if st == nil {
		return &AssistantResult{Err: fmt.Errorf("nil state")}, nil
	}
	if st.Err != nil {
		return p.buildFinalResult(st), nil
	}

	now := time.Now()

	// 1. 保存user消息
	userMsg := &assistant.AIAssistantMessage{
		SessionId: st.SessionID,
		Role:      "user",
		Content:   st.Req.Question,
		CreatedAt: now,
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

	zlog.Info("assistant persist done",
		zap.String("session_id", st.SessionID),
		zap.String("query_id", st.QueryID))

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
