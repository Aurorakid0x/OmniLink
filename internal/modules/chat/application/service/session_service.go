package service

import (
	chatRequest "OmniLink/internal/modules/chat/application/dto/request"
	chatRespond "OmniLink/internal/modules/chat/application/dto/respond"
	chatEntity "OmniLink/internal/modules/chat/domain/entity"
	chatRepository "OmniLink/internal/modules/chat/domain/repository"
	contactRepository "OmniLink/internal/modules/contact/domain/repository"
	userRepository "OmniLink/internal/modules/user/domain/repository"
	"OmniLink/pkg/util"
	"OmniLink/pkg/xerr"
	"OmniLink/pkg/zlog"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type SessionService interface {
	CheckOpenSessionAllowed(req chatRequest.OpenSessionRequest) (bool, error)
	OpenSession(req chatRequest.OpenSessionRequest) (*chatRespond.SessionItem, error)
	GetUserSessionList(ownerID string) ([]chatRespond.SessionItem, error)
	GetGroupSessionList(ownerID string) ([]chatRespond.SessionItem, error)
}

type sessionServiceImpl struct {
	sessionRepo chatRepository.SessionRepository
	contactRepo contactRepository.UserContactRepository
	userRepo    userRepository.UserInfoRepository
	groupRepo   contactRepository.GroupInfoRepository
}

func NewSessionService(sessionRepo chatRepository.SessionRepository, contactRepo contactRepository.UserContactRepository, userRepo userRepository.UserInfoRepository, groupRepo contactRepository.GroupInfoRepository) SessionService {
	return &sessionServiceImpl{
		sessionRepo: sessionRepo,
		contactRepo: contactRepo,
		userRepo:    userRepo,
		groupRepo:   groupRepo,
	}
}

func (s *sessionServiceImpl) GetUserSessionList(ownerID string) ([]chatRespond.SessionItem, error) {
	if ownerID == "" {
		return nil, xerr.New(xerr.BadRequest, xerr.ErrParam.Message)
	}

	sessions, err := s.sessionRepo.ListUserSessionsBySendID(ownerID)
	if err != nil {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	out := make([]chatRespond.SessionItem, 0, len(sessions))
	for i := range sessions {
		sess := sessions[i]
		updatedAt := sess.CreatedAt
		if sess.LastMessageAt.Valid {
			updatedAt = sess.LastMessageAt.Time
		}
		out = append(out, chatRespond.SessionItem{
			SessionId:   sess.Uuid,
			PeerId:      sess.ReceiveId,
			PeerType:    peerTypeOf(sess.ReceiveId),
			PeerName:    sess.ReceiveName,
			PeerAvatar:  sess.Avatar,
			LastMsg:     sess.LastMessage,
			UnreadCount: 0,
			UpdatedAt:   updatedAt.Format(time.RFC3339),
		})
	}

	return out, nil
}

func (s *sessionServiceImpl) GetGroupSessionList(ownerID string) ([]chatRespond.SessionItem, error) {
	if ownerID == "" {
		return nil, xerr.New(xerr.BadRequest, xerr.ErrParam.Message)
	}

	sessions, err := s.sessionRepo.ListGroupSessionsBySendID(ownerID)
	if err != nil {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	contacts, err := s.contactRepo.GetUserContactsByUserID(ownerID)
	if err != nil {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	activeGroups := make(map[string]struct{}, len(contacts))
	for i := range contacts {
		c := contacts[i]
		if c.ContactType != 1 {
			continue
		}
		if c.Status != 0 && c.Status != 5 {
			continue
		}
		if c.ContactId == "" {
			continue
		}
		activeGroups[c.ContactId] = struct{}{}
	}

	out := make([]chatRespond.SessionItem, 0, len(sessions))
	for i := range sessions {
		sess := sessions[i]
		if !strings.HasPrefix(sess.ReceiveId, "G") {
			continue
		}
		if _, ok := activeGroups[sess.ReceiveId]; !ok {
			continue
		}

		updatedAt := sess.CreatedAt
		if sess.LastMessageAt.Valid {
			updatedAt = sess.LastMessageAt.Time
		}
		out = append(out, chatRespond.SessionItem{
			SessionId:   sess.Uuid,
			PeerId:      sess.ReceiveId,
			PeerType:    "G",
			PeerName:    sess.ReceiveName,
			PeerAvatar:  sess.Avatar,
			LastMsg:     sess.LastMessage,
			UnreadCount: 0,
			UpdatedAt:   updatedAt.Format(time.RFC3339),
		})
	}
	return out, nil
}

func (s *sessionServiceImpl) CheckOpenSessionAllowed(req chatRequest.OpenSessionRequest) (bool, error) {
	return s.checkAllowed(req.SendId, req.ReceiveId)
}

func (s *sessionServiceImpl) OpenSession(req chatRequest.OpenSessionRequest) (*chatRespond.SessionItem, error) {
	if req.SendId == "" || req.ReceiveId == "" {
		return nil, xerr.New(xerr.BadRequest, xerr.ErrParam.Message)
	}
	if req.SendId == req.ReceiveId {
		return nil, xerr.New(xerr.BadRequest, "不能和自己创建会话")
	}

	allowed, err := s.checkAllowed(req.SendId, req.ReceiveId)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, xerr.New(xerr.Forbidden, "无权发起会话")
	}

	sessAB, err := s.sessionRepo.GetBySendAndReceive(req.SendId, req.ReceiveId)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			zlog.Error(err.Error())
			return nil, xerr.ErrServerError
		}
		sessAB = nil
	}

	sessBA, err := s.sessionRepo.GetBySendAndReceive(req.ReceiveId, req.SendId)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			zlog.Error(err.Error())
			return nil, xerr.ErrServerError
		}
		sessBA = nil
	}

	if sessAB != nil && sessBA != nil {
		return s.toSessionItem(req.SendId, req.ReceiveId, sessAB), nil
	}

	peerType := peerTypeOf(req.ReceiveId)
	if peerType == "G" {
		// Group session logic
		allowed, err := s.checkAllowed(req.SendId, req.ReceiveId)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, xerr.New(xerr.Forbidden, "无权发起会话")
		}

		// Check if session already exists
		sess, err := s.sessionRepo.GetBySendAndReceive(req.SendId, req.ReceiveId)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			zlog.Error(err.Error())
			return nil, xerr.ErrServerError
		}
		if sess != nil {
			return s.toSessionItem(req.SendId, req.ReceiveId, sess), nil
		}

		// Get Group Info for session creation
		group, err := s.groupRepo.GetGroupInfoByUUID(req.ReceiveId)
		if err != nil {
			return nil, err
		}

		now := time.Now()
		newSess := &chatEntity.Session{
			Uuid:        util.GenerateSessionID(),
			SendId:      req.SendId,
			ReceiveId:   req.ReceiveId,
			ReceiveName: group.Name,
			Avatar:      group.Avatar,
			LastMessage: "",
			CreatedAt:   now,
		}

		if err := s.sessionRepo.Create(newSess); err != nil {
			zlog.Error(err.Error())
			return nil, xerr.ErrServerError
		}

		return s.toSessionItem(req.SendId, req.ReceiveId, newSess), nil
	}

	briefs, err := s.userRepo.GetUserBriefByUUIDs([]string{req.SendId, req.ReceiveId})
	if err != nil {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	var sendName, sendAvatar string
	var sendStatus int8
	foundSend := false

	var peerName, peerAvatar string
	var peerStatus int8
	foundPeer := false

	for i := range briefs {
		b := briefs[i]
		if b.Uuid == req.SendId {
			foundSend = true
			sendAvatar = b.Avatar
			sendStatus = b.Status
			sendName = b.Nickname
			if sendName == "" {
				sendName = b.Username
			}
			continue
		}
		if b.Uuid == req.ReceiveId {
			foundPeer = true
			peerAvatar = b.Avatar
			peerStatus = b.Status
			peerName = b.Nickname
			if peerName == "" {
				peerName = b.Username
			}
			continue
		}
	}

	if !foundPeer {
		return nil, xerr.New(xerr.NotFound, "对方不存在")
	}
	if peerStatus != 0 {
		return nil, xerr.New(xerr.Forbidden, "对方状态异常，无法发起会话")
	}
	if !foundSend {
		return nil, xerr.New(xerr.NotFound, "用户不存在")
	}
	if sendStatus != 0 {
		return nil, xerr.New(xerr.Forbidden, "用户状态异常，无法发起会话")
	}

	now := time.Now()
	toCreate := make([]*chatEntity.Session, 0, 2)

	if sessAB == nil {
		sessAB = &chatEntity.Session{
			Uuid:        util.GenerateSessionID(),
			SendId:      req.SendId,
			ReceiveId:   req.ReceiveId,
			ReceiveName: peerName,
			Avatar:      peerAvatar,
			LastMessage: "",
			CreatedAt:   now,
		}
		toCreate = append(toCreate, sessAB)
	}

	if sessBA == nil {
		sessBA = &chatEntity.Session{
			Uuid:        util.GenerateSessionID(),
			SendId:      req.ReceiveId,
			ReceiveId:   req.SendId,
			ReceiveName: sendName,
			Avatar:      sendAvatar,
			LastMessage: "",
			CreatedAt:   now,
		}
		toCreate = append(toCreate, sessBA)
	}

	if len(toCreate) == 2 {
		if err := s.sessionRepo.CreateMany(toCreate); err != nil {
			zlog.Error(err.Error())
			return nil, xerr.ErrServerError
		}
	} else if len(toCreate) == 1 {
		if err := s.sessionRepo.Create(toCreate[0]); err != nil {
			zlog.Error(err.Error())
			return nil, xerr.ErrServerError
		}
	}

	return s.toSessionItem(req.SendId, req.ReceiveId, sessAB), nil
}

func (s *sessionServiceImpl) checkAllowed(sendID string, receiveID string) (bool, error) {
	if sendID == "" || receiveID == "" {
		return false, xerr.New(xerr.BadRequest, xerr.ErrParam.Message)
	}
	if sendID == receiveID {
		return false, xerr.New(xerr.BadRequest, "不能和自己创建会话")
	}

	peerType := peerTypeOf(receiveID)
	contactType := int8(0)
	if peerType == "G" {
		// Group Check
		// 1. Check group existence and status
		group, err := s.groupRepo.GetGroupInfoByUUID(receiveID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return false, xerr.New(xerr.NotFound, "群组不存在")
			}
			return false, err
		}
		if group.Status != 0 {
			return false, xerr.New(xerr.Forbidden, "群组状态异常")
		}

		// 2. Check if user is a member
		rel, err := s.contactRepo.GetUserContactByUserIDAndContactIDAndType(sendID, receiveID, 1)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return false, xerr.New(xerr.Forbidden, "非群成员")
			}
			zlog.Error(err.Error())
			return false, xerr.ErrServerError
		}
		if rel.Status == 6 || rel.Status == 7 {
			return false, xerr.New(xerr.Forbidden, "非群成员")
		}
		if rel.Status != 0 && rel.Status != 5 {
			return false, xerr.New(xerr.Forbidden, "非正常群成员状态")
		}
		return true, nil
	}

	rel, err := s.contactRepo.GetUserContactByUserIDAndContactIDAndType(sendID, receiveID, contactType)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, xerr.New(xerr.Forbidden, "非好友关系，无法发起会话")
		}
		zlog.Error(err.Error())
		return false, xerr.ErrServerError
	}

	if rel.Status == 2 {
		return false, xerr.New(xerr.Forbidden, "已被对方拉黑，无法发起会话")
	}
	if rel.Status == 1 {
		return false, xerr.New(xerr.Forbidden, "已拉黑对方，先解除拉黑状态才能发起会话")
	}
	if rel.Status != 0 {
		return false, xerr.New(xerr.Forbidden, "非正常好友关系，无法发起会话")
	}

	briefs, err := s.userRepo.GetUserBriefByUUIDs([]string{receiveID})
	if err != nil {
		zlog.Error(err.Error())
		return false, xerr.ErrServerError
	}
	if len(briefs) == 0 {
		return false, xerr.New(xerr.NotFound, "对方不存在")
	}
	if briefs[0].Status != 0 {
		return false, xerr.New(xerr.Forbidden, "对方状态异常，无法发起会话")
	}

	return true, nil
}

func peerTypeOf(id string) string {
	if strings.HasPrefix(id, "G") {
		return "G"
	}
	return "U"
}

func (s *sessionServiceImpl) toSessionItem(sendID string, receiveID string, sess *chatEntity.Session) *chatRespond.SessionItem {
	peerType := peerTypeOf(receiveID)
	updatedAt := sess.CreatedAt
	if sess.LastMessageAt.Valid {
		updatedAt = sess.LastMessageAt.Time
	}

	return &chatRespond.SessionItem{
		SessionId:   sess.Uuid,
		SendId:      sendID,
		ReceiveId:   receiveID,
		ReceiveName: sess.ReceiveName,
		Avatar:      sess.Avatar,
		PeerId:      receiveID,
		PeerType:    peerType,
		PeerName:    sess.ReceiveName,
		PeerAvatar:  sess.Avatar,
		UpdatedAt:   updatedAt.Format(time.RFC3339),
		LastMsg:     sess.LastMessage,
		UnreadCount: 0,
	}
}
