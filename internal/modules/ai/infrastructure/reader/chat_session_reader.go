package reader

import (
	"context"
	"strings"
	"time"

	"OmniLink/internal/modules/chat/domain/entity"
	"OmniLink/internal/modules/chat/domain/repository"
)

// SessionType distinguishes between private and group chats
// 会话类型枚举：区分私聊和群聊
type SessionType int

const (
	SessionTypePrivate SessionType = 1
	SessionTypeGroup   SessionType = 2
)

// ChatSessionItem represents a chat session to be ingested
// 统一的会话对象，抹平了私聊和群聊的差异，方便 Pipeline 处理
type ChatSessionItem struct {
	SessionUUID string      // 会话的唯一标识
	TargetID    string      // 私聊是对方 UserID，群聊是 GroupID
	Type        SessionType // 类型：1=私聊, 2=群聊
	Name        string      // 显示名称（用于日志或元数据）
}

// ChatSessionReader handles reading chat history for RAG ingestion
// 核心读取器：负责从 Chat 模块抽取数据
type ChatSessionReader struct {
	sessionRepo repository.SessionRepository
	messageRepo repository.MessageRepository
}

// NewChatSessionReader creates a new reader instance
// 构造函数：注入 Chat 模块的现成 Repository
func NewChatSessionReader(sRepo repository.SessionRepository, mRepo repository.MessageRepository) *ChatSessionReader {
	return &ChatSessionReader{
		sessionRepo: sRepo,
		messageRepo: mRepo,
	}
}

// ListAllSessions returns all visible sessions for the user (Private + Group)
// 枚举用户能看到的所有会话（私聊 + 群聊）
func (r *ChatSessionReader) ListAllSessions(ctx context.Context, userID string) ([]ChatSessionItem, error) {
	var result []ChatSessionItem

	// 1. 获取私聊会话 (底层逻辑：receive_id like 'U%')
	privateSessions, err := r.sessionRepo.ListUserSessionsBySendID(userID)
	if err != nil {
		return nil, err
	}
	for _, s := range privateSessions {
		result = append(result, ChatSessionItem{
			SessionUUID: s.Uuid,
			TargetID:    s.ReceiveId, // 私聊时 ReceiveId 是对方ID
			Type:        SessionTypePrivate,
			Name:        s.ReceiveName,
		})
	}

	// 2. 获取群聊会话 (底层逻辑：receive_id like 'G%')
	groupSessions, err := r.sessionRepo.ListGroupSessionsBySendID(userID)
	if err != nil {
		return nil, err
	}
	for _, s := range groupSessions {
		result = append(result, ChatSessionItem{
			SessionUUID: s.Uuid,
			TargetID:    s.ReceiveId, // 群聊时 ReceiveId 是 GroupID
			Type:        SessionTypeGroup,
			Name:        s.ReceiveName,
		})
	}

	return result, nil
}

// ReadMessages returns a batch of text messages for a specific session.
// It filters out non-text messages and empty content.
// Note: Messages are returned in DESC order (Newest first) as per underlying repo.
// 分页读取消息，并执行核心过滤逻辑
func (r *ChatSessionReader) ReadMessages(ctx context.Context, userID string, session ChatSessionItem, page, pageSize int, since *time.Time) ([]entity.Message, error) {
	var messages []entity.Message
	var err error

	// 1. 根据会话类型调用不同的底层接口
	if session.Type == SessionTypePrivate {
		// 私聊需要两个人的ID来确定范围
		messages, err = r.messageRepo.ListPrivateMessages(userID, session.TargetID, page, pageSize)
	} else {
		// 群聊只需要 GroupID
		messages, err = r.messageRepo.ListGroupMessages(session.TargetID, page, pageSize)
	}

	if err != nil {
		return nil, err
	}

	// 2. 执行过滤逻辑
	var filtered []entity.Message
	for _, msg := range messages {
		// 过滤 1: 只读文本消息 (Type=0)
		if msg.Type != 0 {
			continue
		}

		// 过滤 2: 过滤空内容
		if strings.TrimSpace(msg.Content) == "" {
			continue
		}

		// 过滤 3: 增量更新检查 (Since)
		// 如果指定了 since 时间，且消息时间不晚于 since，则跳过
		if since != nil {
			if !msg.CreatedAt.After(*since) {
				continue
			}
		}

		filtered = append(filtered, msg)
	}

	return filtered, nil
}
