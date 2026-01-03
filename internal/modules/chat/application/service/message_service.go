package service

import (
	chatRequest "OmniLink/internal/modules/chat/application/dto/request"
	chatRespond "OmniLink/internal/modules/chat/application/dto/respond"
	chatRepository "OmniLink/internal/modules/chat/domain/repository"
	contactRepository "OmniLink/internal/modules/contact/domain/repository"
	"OmniLink/pkg/xerr"
	"OmniLink/pkg/zlog"
	"errors"
	"time"

	"gorm.io/gorm"
)

type MessageService interface {
	GetMessageList(req chatRequest.GetMessageListRequest) ([]chatRespond.MessageItem, error)
	GetGroupMessageList(req chatRequest.GetGroupMessageListRequest, callerID string) ([]chatRespond.MessageItem, error)
}

type messageServiceImpl struct {
	messageRepo chatRepository.MessageRepository
	contactRepo contactRepository.UserContactRepository
}

func NewMessageService(messageRepo chatRepository.MessageRepository, contactRepo contactRepository.UserContactRepository) MessageService {
	return &messageServiceImpl{
		messageRepo: messageRepo,
		contactRepo: contactRepo,
	}
}

func (s *messageServiceImpl) GetMessageList(req chatRequest.GetMessageListRequest) ([]chatRespond.MessageItem, error) {
	if req.UserOneId == "" || req.UserTwoId == "" {
		return nil, xerr.New(xerr.BadRequest, xerr.ErrParam.Message)
	}
	if req.UserOneId == req.UserTwoId {
		return nil, xerr.New(xerr.BadRequest, "不能查询与自己的私聊记录")
	}

	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	rel, err := s.contactRepo.GetUserContactByUserIDAndContactIDAndType(req.UserOneId, req.UserTwoId, 0)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, xerr.New(xerr.Forbidden, "无权查看聊天记录")
		}
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}
	if rel.Status == 2 {
		return nil, xerr.New(xerr.Forbidden, "已被对方拉黑，无法查看聊天记录")
	}
	if rel.Status == 1 {
		return nil, xerr.New(xerr.Forbidden, "已拉黑对方，无法查看聊天记录")
	}
	if rel.Status != 0 {
		return nil, xerr.New(xerr.Forbidden, "无权查看聊天记录")
	}

	msgs, err := s.messageRepo.ListPrivateMessages(req.UserOneId, req.UserTwoId, page, pageSize)
	if err != nil {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	out := make([]chatRespond.MessageItem, 0, len(msgs))
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		out = append(out, chatRespond.MessageItem{
			Uuid:       m.Uuid,
			SessionId:  m.SessionId,
			SendId:     m.SendId,
			SendName:   m.SendName,
			SendAvatar: m.SendAvatar,
			ReceiveId:  m.ReceiveId,
			Type:       m.Type,
			Content:    m.Content,
			Url:        m.Url,
			FileType:   m.FileType,
			FileName:   m.FileName,
			FileSize:   m.FileSize,
			CreatedAt:  m.CreatedAt.Format(time.RFC3339),
		})
	}

	return out, nil
}

func (s *messageServiceImpl) GetGroupMessageList(req chatRequest.GetGroupMessageListRequest, callerID string) ([]chatRespond.MessageItem, error) {
	if req.GroupId == "" {
		return nil, xerr.New(xerr.BadRequest, xerr.ErrParam.Message)
	}

	// 权限检查: 是否群成员
	rel, err := s.contactRepo.GetUserContactByUserIDAndContactIDAndType(callerID, req.GroupId, 1)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, xerr.New(xerr.Forbidden, "非群成员，无权查看消息")
		}
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}
	if rel.Status != 0 {
		return nil, xerr.New(xerr.Forbidden, "非正常群成员状态")
	}

	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	msgs, err := s.messageRepo.ListGroupMessages(req.GroupId, page, pageSize)
	if err != nil {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	out := make([]chatRespond.MessageItem, 0, len(msgs))
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		out = append(out, chatRespond.MessageItem{
			Uuid:       m.Uuid,
			SessionId:  m.SessionId,
			SendId:     m.SendId,
			SendName:   m.SendName,
			SendAvatar: m.SendAvatar,
			ReceiveId:  m.ReceiveId,
			Type:       m.Type,
			Content:    m.Content,
			Url:        m.Url,
			FileType:   m.FileType,
			FileName:   m.FileName,
			FileSize:   m.FileSize,
			CreatedAt:  m.CreatedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}
