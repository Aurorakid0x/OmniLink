package service

import (
	contactRequest "OmniLink/internal/modules/contact/application/dto/request"
	contactRespond "OmniLink/internal/modules/contact/application/dto/respond"
	contactEntity "OmniLink/internal/modules/contact/domain/entity"
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

type ContactService interface {
	GetUserList(req contactRequest.GetUserListRequest) ([]contactRespond.UserListItem, error)
	GetContactInfo(req contactRequest.GetContactInfoRequest) (*contactRespond.GetContactInfoRespond, error)
	ApplyContact(req contactRequest.ApplyContactRequest) (*contactRespond.ApplyContactRespond, error)
	GetNewContactList(req contactRequest.GetNewContactListRequest) ([]contactRespond.NewContactApplyItem, error)
	PassContactApply(req contactRequest.PassContactApplyRequest) error
	RefuseContactApply(req contactRequest.RefuseContactApplyRequest) error
}

type contactServiceImpl struct {
	contactRepo contactRepository.UserContactRepository
	applyRepo   contactRepository.ContactApplyRepository
	userRepo    userRepository.UserInfoRepository
	uow         contactRepository.ContactUnitOfWork
}

func NewContactService(contactRepo contactRepository.UserContactRepository, applyRepo contactRepository.ContactApplyRepository, userRepo userRepository.UserInfoRepository, uow contactRepository.ContactUnitOfWork) ContactService {
	return &contactServiceImpl{
		contactRepo: contactRepo,
		applyRepo:   applyRepo,
		userRepo:    userRepo,
		uow:         uow,
	}
}

func (s *contactServiceImpl) GetUserList(req contactRequest.GetUserListRequest) ([]contactRespond.UserListItem, error) {
	if req.OwnerId == "" {
		return nil, xerr.New(xerr.BadRequest, xerr.ErrParam.Message)
	}

	contacts, err := s.contactRepo.GetUserContactsByUserID(req.OwnerId)
	if err != nil {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	ordered := make([]string, 0, len(contacts))
	seen := make(map[string]struct{}, len(contacts))
	for _, c := range contacts {
		if c.ContactType != 0 {
			continue
		}
		if c.Status != 0 {
			continue
		}
		if c.ContactId == "" {
			continue
		}
		if _, ok := seen[c.ContactId]; ok {
			continue
		}
		seen[c.ContactId] = struct{}{}
		ordered = append(ordered, c.ContactId)
	}

	briefs, err := s.userRepo.GetUserBriefByUUIDs(ordered)
	if err != nil {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	briefMap := make(map[string]struct {
		username string
		nickname string
		avatar   string
		status   int8
	}, len(briefs))
	for _, b := range briefs {
		briefMap[b.Uuid] = struct {
			username string
			nickname string
			avatar   string
			status   int8
		}{
			username: b.Username,
			nickname: b.Nickname,
			avatar:   b.Avatar,
			status:   b.Status,
		}
	}

	out := make([]contactRespond.UserListItem, 0, len(ordered))
	for _, id := range ordered {
		b, ok := briefMap[id]
		if !ok {
			continue
		}
		name := b.nickname
		if name == "" {
			name = b.username
		}
		out = append(out, contactRespond.UserListItem{
			UserId:   id,
			UserName: name,
			Avatar:   b.avatar,
			Status:   b.status,
		})
	}

	return out, nil
}

func (s *contactServiceImpl) PassContactApply(req contactRequest.PassContactApplyRequest) error {
	if req.OwnerId == "" || req.ApplyId == "" {
		return xerr.New(xerr.BadRequest, xerr.ErrParam.Message)
	}

	now := time.Now()
	return s.uow.Transaction(func(applyRepo contactRepository.ContactApplyRepository, contactRepo contactRepository.UserContactRepository, _ contactRepository.GroupInfoRepository) error {
		apply, err := applyRepo.GetContactApplyByUUIDForUpdate(req.ApplyId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return xerr.New(xerr.NotFound, "申请不存在")
			}
			zlog.Error(err.Error())
			return xerr.ErrServerError
		}

		if apply.ContactType != 0 {
			return xerr.New(xerr.BadRequest, "暂不支持群组申请")
		}
		if apply.ContactId != req.OwnerId {
			return xerr.New(xerr.Forbidden, "无权操作该申请")
		}
		if apply.Status == 1 {
			return nil
		}
		if apply.Status == 3 {
			return xerr.New(xerr.Forbidden, "该申请已被拉黑")
		}

		apply.Status = 1
		if err := applyRepo.UpdateContactApply(apply); err != nil {
			zlog.Error(err.Error())
			return xerr.ErrServerError
		}

		upsertFriend := func(userID, contactID string) error {
			rel, err := contactRepo.GetUserContactByUserIDAndContactIDAndType(userID, contactID, 0)
			if err == nil {
				if rel.Status == 0 {
					return nil
				}
				rel.ContactType = 0
				rel.Status = 0
				rel.UpdateAt = now
				return contactRepo.UpdateUserContact(rel)
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}

			newRel := &contactEntity.UserContact{
				UserId:      userID,
				ContactId:   contactID,
				ContactType: 0,
				Status:      0,
				CreatedAt:   now,
				UpdateAt:    now,
			}
			return contactRepo.CreateUserContact(newRel)
		}

		if err := upsertFriend(apply.UserId, apply.ContactId); err != nil {
			zlog.Error(err.Error())
			return xerr.ErrServerError
		}
		if err := upsertFriend(apply.ContactId, apply.UserId); err != nil {
			zlog.Error(err.Error())
			return xerr.ErrServerError
		}

		return nil
	})
}

func (s *contactServiceImpl) RefuseContactApply(req contactRequest.RefuseContactApplyRequest) error {
	if req.OwnerId == "" || req.ApplyId == "" {
		return xerr.New(xerr.BadRequest, xerr.ErrParam.Message)
	}

	return s.uow.Transaction(func(applyRepo contactRepository.ContactApplyRepository, _ contactRepository.UserContactRepository, _ contactRepository.GroupInfoRepository) error {
		apply, err := applyRepo.GetContactApplyByUUIDForUpdate(req.ApplyId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return xerr.New(xerr.NotFound, "申请不存在")
			}
			zlog.Error(err.Error())
			return xerr.ErrServerError
		}

		if apply.ContactType != 0 {
			return xerr.New(xerr.BadRequest, "暂不支持群组申请")
		}
		if apply.ContactId != req.OwnerId {
			return xerr.New(xerr.Forbidden, "无权操作该申请")
		}
		if apply.Status == 2 {
			return nil
		}
		if apply.Status == 1 {
			return xerr.New(xerr.BadRequest, "已通过，无法拒绝")
		}
		if apply.Status == 3 {
			return xerr.New(xerr.Forbidden, "该申请已被拉黑")
		}

		apply.Status = 2
		if err := applyRepo.UpdateContactApply(apply); err != nil {
			zlog.Error(err.Error())
			return xerr.ErrServerError
		}
		return nil
	})
}

func (s *contactServiceImpl) GetContactInfo(req contactRequest.GetContactInfoRequest) (*contactRespond.GetContactInfoRespond, error) {
	if req.OwnerId == "" || req.ContactId == "" {
		return nil, xerr.New(xerr.BadRequest, xerr.ErrParam.Message)
	}

	relation, err := s.contactRepo.GetUserContactByUserIDAndContactID(req.OwnerId, req.ContactId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, xerr.New(xerr.Forbidden, "无权查看该联系人")
		}
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}
	if relation.Status != 0 {
		return nil, xerr.New(xerr.Forbidden, "无权查看该联系人")
	}

	if relation.ContactType == 0 {
		users, err := s.userRepo.GetUserContactInfoByUUIDs([]string{req.ContactId})
		if err != nil {
			zlog.Error(err.Error())
			return nil, xerr.ErrServerError
		}
		if len(users) == 0 {
			return nil, xerr.New(xerr.NotFound, "联系人不存在")
		}

		u := users[0]
		name := u.Nickname
		if name == "" {
			name = u.Username
		}

		return &contactRespond.GetContactInfoRespond{
			ContactId:        u.Uuid,
			ContactName:      name,
			ContactAvatar:    u.Avatar,
			ContactSignature: u.Signature,
			Gender:           u.Gender,
			Birthday:         u.Birthday,
		}, nil
	}

	return &contactRespond.GetContactInfoRespond{
		ContactId:        req.ContactId,
		ContactName:      req.ContactId,
		ContactAvatar:    "",
		ContactSignature: "",
		Gender:           -1,
		Birthday:         "",
	}, nil
}

func (s *contactServiceImpl) ApplyContact(req contactRequest.ApplyContactRequest) (*contactRespond.ApplyContactRespond, error) {
	if req.OwnerId == "" || req.ContactId == "" {
		return nil, xerr.New(xerr.BadRequest, xerr.ErrParam.Message)
	}
	if req.OwnerId == req.ContactId {
		return nil, xerr.New(xerr.BadRequest, "不能添加自己")
	}

	// 自动识别类型
	contactType := int8(0)
	if strings.HasPrefix(req.ContactId, "G") {
		contactType = 1
	}

	if contactType == 0 {
		briefs, err := s.userRepo.GetUserBriefByUUIDs([]string{req.ContactId})
		if err != nil {
			zlog.Error(err.Error())
			return nil, xerr.ErrServerError
		}
		if len(briefs) == 0 {
			return nil, xerr.New(xerr.NotFound, "用户不存在")
		}
		if briefs[0].Status != 0 {
			return nil, xerr.New(xerr.Forbidden, "用户不可用")
		}
	}

	// 检查关系状态
	if rel, err := s.contactRepo.GetUserContactByUserIDAndContactID(req.OwnerId, req.ContactId); err == nil {
		if rel.ContactType == contactType {
			switch rel.Status {
			case 0:
				return nil, xerr.New(xerr.BadRequest, "已是好友")
			case 1:
				return nil, xerr.New(xerr.BadRequest, "您已将对方拉黑")
			case 2:
				return nil, xerr.New(xerr.Forbidden, "对方已将您拉黑")
			}
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	now := time.Now()
	apply, err := s.applyRepo.GetContactApplyByUserIDAndContactID(req.OwnerId, req.ContactId, contactType)
	if err == nil {
		apply.Status = 0
		apply.Message = req.Message
		apply.LastApplyAt = now
		if apply.Uuid == "" {
			apply.Uuid = util.GenerateApplyID()
		}
		if err := s.applyRepo.UpdateContactApply(apply); err != nil {
			zlog.Error(err.Error())
			return nil, xerr.ErrServerError
		}
		return &contactRespond.ApplyContactRespond{ApplyId: apply.Uuid}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	newApply := contactEntity.ContactApply{
		Uuid:        util.GenerateApplyID(),
		UserId:      req.OwnerId,
		ContactId:   req.ContactId,
		ContactType: contactType,
		Status:      0,
		Message:     req.Message,
		LastApplyAt: now,
	}
	if err := s.applyRepo.CreateContactApply(&newApply); err != nil {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	return &contactRespond.ApplyContactRespond{ApplyId: newApply.Uuid}, nil
}

func (s *contactServiceImpl) GetNewContactList(req contactRequest.GetNewContactListRequest) ([]contactRespond.NewContactApplyItem, error) {
	if req.OwnerId == "" {
		return nil, xerr.New(xerr.BadRequest, xerr.ErrParam.Message)
	}

	applies, err := s.applyRepo.ListPendingAppliesByContactID(req.OwnerId)
	if err != nil {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}
	if len(applies) == 0 {
		return []contactRespond.NewContactApplyItem{}, nil
	}

	userIDs := make([]string, 0, len(applies))
	seen := make(map[string]struct{}, len(applies))
	for _, a := range applies {
		if a.ContactType != 0 {
			continue
		}
		if a.UserId == "" {
			continue
		}
		if _, ok := seen[a.UserId]; ok {
			continue
		}
		seen[a.UserId] = struct{}{}
		userIDs = append(userIDs, a.UserId)
	}

	briefs, err := s.userRepo.GetUserBriefByUUIDs(userIDs)
	if err != nil {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}
	briefMap := make(map[string]struct {
		username string
		nickname string
		avatar   string
	}, len(briefs))
	for _, b := range briefs {
		briefMap[b.Uuid] = struct {
			username string
			nickname string
			avatar   string
		}{
			username: b.Username,
			nickname: b.Nickname,
			avatar:   b.Avatar,
		}
	}

	out := make([]contactRespond.NewContactApplyItem, 0, len(applies))
	for _, a := range applies {
		if a.ContactType != 0 {
			continue
		}
		b, ok := briefMap[a.UserId]
		item := contactRespond.NewContactApplyItem{
			Uuid:        a.Uuid,
			UserId:      a.UserId,
			Username:    "",
			Nickname:    "",
			Avatar:      "",
			Message:     a.Message,
			Status:      a.Status,
			LastApplyAt: a.LastApplyAt.Format("2006-01-02 15:04:05"),
		}
		if ok {
			item.Username = b.username
			item.Nickname = b.nickname
			item.Avatar = b.avatar
		}
		out = append(out, item)
	}

	return out, nil
}
