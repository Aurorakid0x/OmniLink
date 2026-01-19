package service

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	aiRequest "OmniLink/internal/modules/ai/application/dto/request"
	aiIngest "OmniLink/internal/modules/ai/application/service"
	chatRequest "OmniLink/internal/modules/chat/application/dto/request"
	chatRespond "OmniLink/internal/modules/chat/application/dto/respond"
	chatEntity "OmniLink/internal/modules/chat/domain/entity"
	chatRepository "OmniLink/internal/modules/chat/domain/repository"
	contactRepository "OmniLink/internal/modules/contact/domain/repository"
	userRepository "OmniLink/internal/modules/user/domain/repository"
	"OmniLink/pkg/util"
	"OmniLink/pkg/xerr"
	"OmniLink/pkg/zlog"

	"gorm.io/gorm"
)

type RealtimeService interface {
	SendPrivateMessage(senderID string, req chatRequest.SendMessageRequest) (*chatRespond.MessageItem, *chatRespond.MessageItem, error)
	SendGroupMessage(senderID string, req chatRequest.SendMessageRequest) ([]string, *chatRespond.MessageItem, error)
}

type realtimeServiceImpl struct {
	messageRepo chatRepository.MessageRepository
	sessionRepo chatRepository.SessionRepository
	contactRepo contactRepository.UserContactRepository
	userRepo    userRepository.UserInfoRepository
	groupRepo   contactRepository.GroupInfoRepository
	aiIngest    aiIngest.AsyncIngestService
}

func NewRealtimeService(
	messageRepo chatRepository.MessageRepository,
	sessionRepo chatRepository.SessionRepository,
	contactRepo contactRepository.UserContactRepository,
	userRepo userRepository.UserInfoRepository,
	groupRepo contactRepository.GroupInfoRepository,
	aiIngestSvc aiIngest.AsyncIngestService,
) RealtimeService {
	return &realtimeServiceImpl{
		messageRepo: messageRepo,
		sessionRepo: sessionRepo,
		contactRepo: contactRepo,
		userRepo:    userRepo,
		groupRepo:   groupRepo,
		aiIngest:    aiIngestSvc,
	}
}

func (s *realtimeServiceImpl) SendPrivateMessage(senderID string, req chatRequest.SendMessageRequest) (*chatRespond.MessageItem, *chatRespond.MessageItem, error) {
	if senderID == "" || req.ReceiveId == "" {
		return nil, nil, xerr.New(xerr.BadRequest, xerr.ErrParam.Message)
	}
	if senderID == req.ReceiveId {
		return nil, nil, xerr.New(xerr.BadRequest, "不能给自己发消息")
	}
	if req.Type == 0 && req.Content == "" {
		return nil, nil, xerr.New(xerr.BadRequest, "消息内容不能为空")
	}

	rel, err := s.contactRepo.GetUserContactByUserIDAndContactIDAndType(senderID, req.ReceiveId, 0)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, xerr.New(xerr.Forbidden, "无权发送消息")
		}
		zlog.Error(err.Error())
		return nil, nil, xerr.ErrServerError
	}
	if rel.Status == 2 {
		return nil, nil, xerr.New(xerr.Forbidden, "已被对方拉黑，无法发送消息")
	}
	if rel.Status == 1 {
		return nil, nil, xerr.New(xerr.Forbidden, "已拉黑对方，无法发送消息")
	}
	if rel.Status != 0 {
		return nil, nil, xerr.New(xerr.Forbidden, "无权发送消息")
	}

	sessSender, err := s.sessionRepo.GetBySendAndReceive(senderID, req.ReceiveId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, xerr.New(xerr.BadRequest, "会话不存在，请先创建会话")
		}
		zlog.Error(err.Error())
		return nil, nil, xerr.ErrServerError
	}
	if req.SessionId != "" && req.SessionId != sessSender.Uuid {
		return nil, nil, xerr.New(xerr.BadRequest, "session_id 不匹配")
	}

	sessReceiver, err := s.sessionRepo.GetBySendAndReceive(req.ReceiveId, senderID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, xerr.New(xerr.BadRequest, "对方会话不存在，请对方先创建会话")
		}
		zlog.Error(err.Error())
		return nil, nil, xerr.ErrServerError
	}

	briefs, err := s.userRepo.GetUserBriefByUUIDs([]string{senderID})
	if err != nil {
		zlog.Error(err.Error())
		return nil, nil, xerr.ErrServerError
	}
	if len(briefs) == 0 || briefs[0].Status != 0 {
		return nil, nil, xerr.New(xerr.Forbidden, "用户状态异常，无法发送消息")
	}

	sendName := briefs[0].Nickname
	if sendName == "" {
		sendName = briefs[0].Username
	}

	now := time.Now()
	msg := &chatEntity.Message{
		Uuid:       util.GenerateMessageID(),
		SessionId:  sessSender.Uuid,
		Type:       req.Type,
		Content:    req.Content,
		Url:        req.Url,
		SendId:     senderID,
		SendName:   sendName,
		SendAvatar: briefs[0].Avatar,
		ReceiveId:  req.ReceiveId,
		FileType:   req.FileType,
		FileName:   req.FileName,
		FileSize:   req.FileSize,
		Status:     1,
		CreatedAt:  now,
		SendAt:     sql.NullTime{Time: now, Valid: true},
	}

	if err := s.messageRepo.Create(msg); err != nil {
		zlog.Error(err.Error())
		return nil, nil, xerr.ErrServerError
	}

	lastMessage := msg.Content
	if msg.Type != 0 {
		lastMessage = "[多媒体消息]"
	}
	_ = s.sessionRepo.UpdateLastMessageBySendAndReceive(senderID, req.ReceiveId, lastMessage, now)
	_ = s.sessionRepo.UpdateLastMessageBySendAndReceive(req.ReceiveId, senderID, lastMessage, now)

	if s.aiIngest != nil && msg.Type == 0 && strings.TrimSpace(msg.Content) != "" {
		since := msg.CreatedAt.Add(-5 * time.Second)
		_ = s.aiIngest.EnqueueChatMessagesPage(context.Background(), aiRequest.ChatMessagesPageRequest{
			TenantUserID: senderID,
			SessionUUID:  sessSender.Uuid,
			SessionType:  1,
			SessionName:  sessSender.ReceiveName,
			TargetID:     req.ReceiveId,
			Page:         1,
			PageSize:     50,
			Since:        &since,
			SourceType:   "chat_private",
			SourceKey:    req.ReceiveId,
			DedupExtra:   msg.Uuid,
		})
		_ = s.aiIngest.EnqueueChatMessagesPage(context.Background(), aiRequest.ChatMessagesPageRequest{
			TenantUserID: req.ReceiveId,
			SessionUUID:  sessReceiver.Uuid,
			SessionType:  1,
			SessionName:  sessReceiver.ReceiveName,
			TargetID:     senderID,
			Page:         1,
			PageSize:     50,
			Since:        &since,
			SourceType:   "chat_private",
			SourceKey:    senderID,
			DedupExtra:   msg.Uuid,
		})
	}

	senderItem := &chatRespond.MessageItem{
		Uuid:       msg.Uuid,
		SessionId:  sessSender.Uuid,
		SendId:     msg.SendId,
		SendName:   msg.SendName,
		SendAvatar: msg.SendAvatar,
		ReceiveId:  msg.ReceiveId,
		Type:       msg.Type,
		Content:    msg.Content,
		Url:        msg.Url,
		FileType:   msg.FileType,
		FileName:   msg.FileName,
		FileSize:   msg.FileSize,
		CreatedAt:  msg.CreatedAt.Format(time.RFC3339),
	}
	receiverItem := &chatRespond.MessageItem{
		Uuid:       msg.Uuid,
		SessionId:  sessReceiver.Uuid,
		SendId:     msg.SendId,
		SendName:   msg.SendName,
		SendAvatar: msg.SendAvatar,
		ReceiveId:  msg.ReceiveId,
		Type:       msg.Type,
		Content:    msg.Content,
		Url:        msg.Url,
		FileType:   msg.FileType,
		FileName:   msg.FileName,
		FileSize:   msg.FileSize,
		CreatedAt:  msg.CreatedAt.Format(time.RFC3339),
	}

	return senderItem, receiverItem, nil
}

func (s *realtimeServiceImpl) SendGroupMessage(senderID string, req chatRequest.SendMessageRequest) ([]string, *chatRespond.MessageItem, error) {
	if senderID == "" || req.ReceiveId == "" {
		return nil, nil, xerr.New(xerr.BadRequest, xerr.ErrParam.Message)
	}

	// 1. 校验群组
	group, err := s.groupRepo.GetGroupInfoByUUID(req.ReceiveId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, xerr.New(xerr.NotFound, "群组不存在")
		}
		zlog.Error(err.Error())
		return nil, nil, xerr.ErrServerError
	}
	if group.Status != 0 {
		return nil, nil, xerr.New(xerr.Forbidden, "群组状态异常")
	}

	// 2. 校验发送者权限
	rel, err := s.contactRepo.GetUserContactByUserIDAndContactIDAndType(senderID, req.ReceiveId, 1)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, xerr.New(xerr.Forbidden, "非群成员，无法发送消息")
		}
		zlog.Error(err.Error())
		return nil, nil, xerr.ErrServerError
	}
	if rel.Status != 0 {
		return nil, nil, xerr.New(xerr.Forbidden, "无权发送消息")
	}

	// 3. 获取所有群成员
	members, err := s.contactRepo.GetGroupMembers(req.ReceiveId)
	if err != nil {
		zlog.Error(err.Error())
		return nil, nil, xerr.ErrServerError
	}
	memberIDs := make([]string, 0, len(members))
	for _, m := range members {
		memberIDs = append(memberIDs, m.UserId)
	}

	// 4. 获取发送者信息
	briefs, err := s.userRepo.GetUserBriefByUUIDs([]string{senderID})
	if err != nil {
		zlog.Error(err.Error())
		return nil, nil, xerr.ErrServerError
	}
	if len(briefs) == 0 {
		return nil, nil, xerr.New(xerr.Forbidden, "用户异常")
	}
	sendName := briefs[0].Nickname
	if sendName == "" {
		sendName = briefs[0].Username
	}

	// 5. 消息落库
	now := time.Now()
	msg := &chatEntity.Message{
		Uuid:       util.GenerateMessageID(),
		SessionId:  "", // 群消息不绑定单一 session_id
		Type:       req.Type,
		Content:    req.Content,
		Url:        req.Url,
		SendId:     senderID,
		SendName:   sendName,
		SendAvatar: briefs[0].Avatar,
		ReceiveId:  req.ReceiveId,
		FileType:   req.FileType,
		FileName:   req.FileName,
		FileSize:   req.FileSize,
		Status:     1,
		CreatedAt:  now,
		SendAt:     sql.NullTime{Time: now, Valid: true},
	}

	if err := s.messageRepo.Create(msg); err != nil {
		zlog.Error(err.Error())
		return nil, nil, xerr.ErrServerError
	}

	// 6. 更新或创建会话
	lastMessage := msg.Content
	if msg.Type != 0 {
		lastMessage = "[多媒体消息]"
	}

	sessUUIDByUser := make(map[string]string, len(memberIDs))
	for _, uid := range memberIDs {
		sess, err := s.sessionRepo.GetBySendAndReceive(uid, req.ReceiveId)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			newSess := &chatEntity.Session{
				Uuid:          util.GenerateSessionID(),
				SendId:        uid,
				ReceiveId:     req.ReceiveId,
				ReceiveName:   group.Name,
				Avatar:        group.Avatar,
				LastMessage:   lastMessage,
				LastMessageAt: sql.NullTime{Time: now, Valid: true},
				CreatedAt:     now,
			}
			_ = s.sessionRepo.Create(newSess)
			sessUUIDByUser[uid] = newSess.Uuid
		} else {
			if err == nil && sess != nil {
				sessUUIDByUser[uid] = sess.Uuid
			}
			_ = s.sessionRepo.UpdateLastMessageBySendAndReceive(uid, req.ReceiveId, lastMessage, now)
		}
	}

	if s.aiIngest != nil && msg.Type == 0 && strings.TrimSpace(msg.Content) != "" {
		since := msg.CreatedAt.Add(-5 * time.Second)
		for _, uid := range memberIDs {
			sessUUID := strings.TrimSpace(sessUUIDByUser[uid])
			if sessUUID == "" {
				continue
			}
			_ = s.aiIngest.EnqueueChatMessagesPage(context.Background(), aiRequest.ChatMessagesPageRequest{
				TenantUserID: uid,
				SessionUUID:  sessUUID,
				SessionType:  2,
				SessionName:  group.Name,
				TargetID:     req.ReceiveId,
				Page:         1,
				PageSize:     50,
				Since:        &since,
				SourceType:   "chat_group",
				SourceKey:    req.ReceiveId,
				DedupExtra:   msg.Uuid,
			})
		}
	}

	item := &chatRespond.MessageItem{
		Uuid:       msg.Uuid,
		SessionId:  "",
		SendId:     msg.SendId,
		SendName:   msg.SendName,
		SendAvatar: msg.SendAvatar,
		ReceiveId:  msg.ReceiveId,
		Type:       msg.Type,
		Content:    msg.Content,
		Url:        msg.Url,
		FileType:   msg.FileType,
		FileName:   msg.FileName,
		FileSize:   msg.FileSize,
		CreatedAt:  msg.CreatedAt.Format(time.RFC3339),
	}

	return memberIDs, item, nil
}
