package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"OmniLink/internal/modules/ai/application/dto/request"
	"OmniLink/internal/modules/ai/application/dto/respond"
	"OmniLink/internal/modules/ai/domain/repository"
	"OmniLink/internal/modules/ai/infrastructure/pipeline"
)

// AssistantService 全局AI助手服务接口
type AssistantService interface {
	// Chat 非流式聊天
	Chat(ctx context.Context, req request.AssistantChatRequest, tenantUserID string) (*respond.AssistantChatRespond, error)

	// ChatStream 流式聊天（返回channel用于SSE）
	ChatStream(ctx context.Context, req request.AssistantChatRequest, tenantUserID string) (<-chan StreamEvent, error)

	// ListSessions 获取会话列表
	ListSessions(ctx context.Context, tenantUserID string, limit, offset int) (*respond.AssistantSessionListRespond, error)

	// ListAgents 获取Agent列表
	ListAgents(ctx context.Context, tenantUserID string, limit, offset int) (*respond.AssistantAgentListRespond, error)

	// GetSessionMessages 获取会话历史消息列表
	GetSessionMessages(ctx context.Context, sessionID, tenantUserID string, limit, offset int) (*respond.AssistantMessageListRespond, error)
}

// StreamEvent SSE流式事件
type StreamEvent struct {
	Event string      // "delta" or "done" or "error"
	Data  interface{} // delta: {token: "..."}, done: AssistantStreamDoneEvent, error: {error: "..."}
}

type assistantServiceImpl struct {
	sessionRepo repository.AssistantSessionRepository
	messageRepo repository.AssistantMessageRepository
	agentRepo   repository.AgentRepository
	pipeline    *pipeline.AssistantPipeline
}

// NewAssistantService 创建AssistantService
func NewAssistantService(
	sessionRepo repository.AssistantSessionRepository,
	messageRepo repository.AssistantMessageRepository,
	agentRepo repository.AgentRepository,
	pipe *pipeline.AssistantPipeline,
) AssistantService {
	return &assistantServiceImpl{
		sessionRepo: sessionRepo,
		messageRepo: messageRepo,
		agentRepo:   agentRepo,
		pipeline:    pipe,
	}
}

func (s *assistantServiceImpl) Chat(ctx context.Context, req request.AssistantChatRequest, tenantUserID string) (*respond.AssistantChatRespond, error) {
	tenantUserID = strings.TrimSpace(tenantUserID)
	if tenantUserID == "" {
		return nil, fmt.Errorf("tenant_user_id is required")
	}
	if strings.TrimSpace(req.Question) == "" {
		return nil, fmt.Errorf("question is required")
	}

	pipeReq := &pipeline.AssistantRequest{
		SessionID:    strings.TrimSpace(req.SessionID),
		TenantUserID: tenantUserID,
		Question:     strings.TrimSpace(req.Question),
		TopK:         req.TopK,
		Scope:        strings.TrimSpace(req.Scope),
		SourceKeys:   req.SourceKeys,
		AgentID:      strings.TrimSpace(req.AgentID),
	}

	result, err := s.pipeline.Execute(ctx, pipeReq)
	if err != nil {
		return nil, err
	}
	if result.Err != nil {
		return nil, result.Err
	}

	return &respond.AssistantChatRespond{
		SessionID: result.SessionID,
		Answer:    result.Answer,
		Citations: result.Citations,
		QueryID:   result.QueryID,
		Timing:    result.Timing,
	}, nil
}

func (s *assistantServiceImpl) ChatStream(ctx context.Context, req request.AssistantChatRequest, tenantUserID string) (<-chan StreamEvent, error) {
	tenantUserID = strings.TrimSpace(tenantUserID)
	if tenantUserID == "" {
		return nil, fmt.Errorf("tenant_user_id is required")
	}
	if strings.TrimSpace(req.Question) == "" {
		return nil, fmt.Errorf("question is required")
	}

	eventChan := make(chan StreamEvent, 100)

	go func() {
		defer close(eventChan)

		pipeReq := &pipeline.AssistantRequest{
			SessionID:    strings.TrimSpace(req.SessionID),
			TenantUserID: tenantUserID,
			Question:     strings.TrimSpace(req.Question),
			TopK:         req.TopK,
			Scope:        strings.TrimSpace(req.Scope),
			SourceKeys:   req.SourceKeys,
			AgentID:      strings.TrimSpace(req.AgentID),
		}

		// 执行流式Pipeline
		streamReader, st, err := s.pipeline.ExecuteStream(ctx, pipeReq)
		if err != nil {
			eventChan <- StreamEvent{Event: "error", Data: map[string]string{"error": err.Error()}}
			return
		}

		// 读取流式输出
		llmStart := time.Now()
		fullAnswer := ""
		for {
			chunk, err := streamReader.Recv()
			if err != nil {
				break // EOF or error
			}
			token := chunk.Content
			fullAnswer += token
			eventChan <- StreamEvent{Event: "delta", Data: map[string]string{"token": token}}
		}
		llmMs := time.Since(llmStart).Milliseconds()

		// 持久化结果
		result, err := s.pipeline.PersistStreamResult(ctx, st, fullAnswer, llmMs)
		if err != nil {
			eventChan <- StreamEvent{Event: "error", Data: map[string]string{"error": err.Error()}}
			return
		}

		// 发送done事件
		doneEvent := respond.AssistantStreamDoneEvent{
			SessionID: result.SessionID,
			Answer:    result.Answer,
			Citations: result.Citations,
			QueryID:   result.QueryID,
			Timing:    result.Timing,
		}
		eventChan <- StreamEvent{Event: "done", Data: doneEvent}
	}()

	return eventChan, nil
}

func (s *assistantServiceImpl) ListSessions(ctx context.Context, tenantUserID string, limit, offset int) (*respond.AssistantSessionListRespond, error) {
	sessions, err := s.sessionRepo.ListSessions(ctx, tenantUserID, limit, offset)
	if err != nil {
		return nil, err
	}

	items := make([]*respond.AssistantSessionItem, 0, len(sessions))
	for _, sess := range sessions {
		lastMessage := ""
		summary := ""
		if s.messageRepo != nil {
			msgs, err := s.messageRepo.ListRecentMessages(ctx, sess.SessionId, 1)
			if err != nil {
				return nil, err
			}
			if len(msgs) > 0 {
				lastMessage = msgs[0].Content
				summary = truncateSummary(lastMessage, 80)
			}
		}
		items = append(items, &respond.AssistantSessionItem{
			SessionID:   sess.SessionId,
			Title:       sess.Title,
			AgentID:     sess.AgentId,
			UpdatedAt:   sess.UpdatedAt,
			LastMessage: lastMessage,
			Summary:     summary,
		})
	}

	return &respond.AssistantSessionListRespond{
		Sessions: items,
		Total:    len(items),
	}, nil
}

func (s *assistantServiceImpl) ListAgents(ctx context.Context, tenantUserID string, limit, offset int) (*respond.AssistantAgentListRespond, error) {
	if s.agentRepo == nil {
		return &respond.AssistantAgentListRespond{Agents: []*respond.AssistantAgentItem{}, Total: 0}, nil
	}

	agents, err := s.agentRepo.ListAgents(ctx, tenantUserID, limit, offset)
	if err != nil {
		return nil, err
	}

	items := make([]*respond.AssistantAgentItem, 0, len(agents))
	for _, ag := range agents {
		items = append(items, &respond.AssistantAgentItem{
			AgentID:     ag.AgentId,
			Name:        ag.Name,
			Description: ag.Description,
			Status:      ag.Status,
			OwnerType:   ag.OwnerType,
		})
	}

	return &respond.AssistantAgentListRespond{
		Agents: items,
		Total:  len(items),
	}, nil
}

func truncateSummary(content string, maxLen int) string {
	content = strings.TrimSpace(content)
	if content == "" || maxLen <= 0 {
		return ""
	}
	runes := []rune(content)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "..."
	}
	return content
}

func parseCitationsJSON(raw string) []respond.CitationEntry {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" {
		return nil
	}
	var citations []respond.CitationEntry
	if err := json.Unmarshal([]byte(raw), &citations); err == nil {
		return citations
	}
	var single respond.CitationEntry
	if err := json.Unmarshal([]byte(raw), &single); err == nil {
		if single.ChunkID != "" || single.SourceKey != "" || single.Content != "" {
			return []respond.CitationEntry{single}
		}
	}
	return nil
}

func (s *assistantServiceImpl) GetSessionMessages(ctx context.Context, sessionID, tenantUserID string, limit, offset int) (*respond.AssistantMessageListRespond, error) {
	sessionID = strings.TrimSpace(sessionID)
	tenantUserID = strings.TrimSpace(tenantUserID)
	if sessionID == "" {
		return nil, fmt.Errorf("session_id is required")
	}
	if tenantUserID == "" {
		return nil, fmt.Errorf("tenant_user_id is required")
	}
	if s.messageRepo == nil {
		return nil, fmt.Errorf("message repository is nil")
	}

	session, err := s.sessionRepo.GetSessionByID(ctx, sessionID, tenantUserID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	if session == nil {
		return nil, fmt.Errorf("session not found or access denied")
	}

	if limit <= 0 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	totalCount, err := s.messageRepo.CountSessionMessages(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to count messages: %w", err)
	}

	messages, err := s.messageRepo.ListMessages(ctx, sessionID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list messages: %w", err)
	}

	items := make([]*respond.AssistantMessageItem, 0, len(messages))
	for _, msg := range messages {
		item := &respond.AssistantMessageItem{
			Role:      msg.Role,
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt,
		}

		if msg.Role == "assistant" && msg.CitationsJson != "" {
			item.Citations = parseCitationsJSON(msg.CitationsJson)
		}

		if msg.TokensJson != "" {
			var tokensMap map[string]int
			if err := json.Unmarshal([]byte(msg.TokensJson), &tokensMap); err == nil {
				item.TokensPrompt = tokensMap["prompt_tokens"]
				item.TokensAnswer = tokensMap["answer_tokens"]
				item.TokensTotal = tokensMap["total_tokens"]
			}
		}

		items = append(items, item)
	}

	return &respond.AssistantMessageListRespond{
		SessionID: sessionID,
		Messages:  items,
		Total:     int(totalCount),
	}, nil
}
