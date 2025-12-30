package service

import (
	contactRequest "OmniLink/internal/modules/contact/application/dto/request"
	contactRespond "OmniLink/internal/modules/contact/application/dto/respond"
	contactRepository "OmniLink/internal/modules/contact/domain/repository"
	userRepository "OmniLink/internal/modules/user/domain/repository"
	"OmniLink/pkg/xerr"
	"OmniLink/pkg/zlog"
	"errors"

	"gorm.io/gorm"
)

type ContactService interface {
	GetUserList(req contactRequest.GetUserListRequest) ([]contactRespond.UserListItem, error)
	GetContactInfo(req contactRequest.GetContactInfoRequest) (*contactRespond.GetContactInfoRespond, error)
}

type contactServiceImpl struct {
	contactRepo contactRepository.UserContactRepository
	userRepo    userRepository.UserInfoRepository
}

func NewContactService(contactRepo contactRepository.UserContactRepository, userRepo userRepository.UserInfoRepository) ContactService {
	return &contactServiceImpl{
		contactRepo: contactRepo,
		userRepo:    userRepo,
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
