package service

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	contactRequest "OmniLink/internal/modules/contact/application/dto/request"
	contactRespond "OmniLink/internal/modules/contact/application/dto/respond"
	contactEntity "OmniLink/internal/modules/contact/domain/entity"
	contactRepository "OmniLink/internal/modules/contact/domain/repository"

	//userEntity "OmniLink/internal/modules/user/domain/entity"
	userRepository "OmniLink/internal/modules/user/domain/repository"
	"OmniLink/pkg/util"
	"OmniLink/pkg/xerr"
	"OmniLink/pkg/zlog"

	"gorm.io/gorm"
)

type GroupService interface {
	CreateGroup(req contactRequest.CreateGroupRequest) (*contactRespond.CreateGroupRespond, error)
	GetGroupInfo(req contactRequest.GetGroupInfoRequest) (*contactRespond.CreateGroupRespond, error)
	GetGroupMemberList(req contactRequest.GetGroupMemberListRequest) ([]*contactRespond.GroupMemberRespond, error)
	InviteGroupMembers(req contactRequest.InviteGroupMembersRequest) error
}

type groupServiceImpl struct {
	userRepo userRepository.UserInfoRepository
	uow      contactRepository.ContactUnitOfWork
}

func NewGroupService(
	_ contactRepository.UserContactRepository,
	_ contactRepository.GroupInfoRepository,
	userRepo userRepository.UserInfoRepository,
	uow contactRepository.ContactUnitOfWork,
) GroupService {
	return &groupServiceImpl{
		userRepo: userRepo,
		uow:      uow,
	}
}

func (s *groupServiceImpl) CreateGroup(req contactRequest.CreateGroupRequest) (*contactRespond.CreateGroupRespond, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Notice = strings.TrimSpace(req.Notice)

	if req.OwnerId == "" || req.Name == "" {
		return nil, xerr.New(xerr.BadRequest, xerr.ErrParam.Message)
	}

	memberSet := make(map[string]struct{}, len(req.MemberIds)+1)
	memberIDs := make([]string, 0, len(req.MemberIds)+1)

	addMember := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := memberSet[id]; ok {
			return
		}
		memberSet[id] = struct{}{}
		memberIDs = append(memberIDs, id)
	}

	addMember(req.OwnerId)
	for _, id := range req.MemberIds {
		addMember(id)
	}

	briefs, err := s.userRepo.GetUserBriefByUUIDs(memberIDs)
	if err != nil {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	found := make(map[string]struct {
		status int8
	}, len(briefs))
	for _, b := range briefs {
		found[b.Uuid] = struct {
			status int8
		}{status: b.Status}
	}

	for _, id := range memberIDs {
		b, ok := found[id]
		if !ok {
			return nil, xerr.New(xerr.NotFound, "成员不存在")
		}
		if b.status != 0 {
			return nil, xerr.New(xerr.Forbidden, "成员状态异常")
		}
	}

	now := time.Now()
	groupID := util.GenerateGroupID()

	membersJSON, err := json.Marshal(memberIDs)
	if err != nil {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	const defaultGroupAvatar = "https://cube.elemecdn.com/0/88/03b0d39583f48206768a7534e55bcpng.png"

	group := &contactEntity.GroupInfo{
		Uuid:      groupID,
		Name:      req.Name,
		Notice:    req.Notice,
		Members:   membersJSON,
		MemberCnt: len(memberIDs),
		OwnerId:   req.OwnerId,
		AddMode:   0,
		Avatar:    defaultGroupAvatar,
		Status:    0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	err = s.uow.Transaction(func(_ contactRepository.ContactApplyRepository, contactRepo contactRepository.UserContactRepository, groupRepo contactRepository.GroupInfoRepository) error {
		if err := groupRepo.CreateGroupInfo(group); err != nil {
			zlog.Error(err.Error())
			return xerr.ErrServerError
		}

		upsertMemberRel := func(userID string) error {
			rel, err := contactRepo.GetUserContactByUserIDAndContactIDAndType(userID, groupID, 1)
			if err == nil {
				if rel.Status == 0 {
					return nil
				}
				rel.ContactType = 1
				rel.Status = 0
				rel.UpdateAt = now
				if err := contactRepo.UpdateUserContact(rel); err != nil {
					zlog.Error(err.Error())
					return xerr.ErrServerError
				}
				return nil
			}

			if !errors.Is(err, gorm.ErrRecordNotFound) {
				zlog.Error(err.Error())
				return xerr.ErrServerError
			}

			newRel := &contactEntity.UserContact{
				UserId:      userID,
				ContactId:   groupID,
				ContactType: 1,
				Status:      0,
				CreatedAt:   now,
				UpdateAt:    now,
			}
			if err := contactRepo.CreateUserContact(newRel); err != nil {
				zlog.Error(err.Error())
				return xerr.ErrServerError
			}
			return nil
		}
		//todo：有时间改成用户可以设置自己的入群模式——1.被邀请入群无需同意，2.被邀请入群需要自己同意，
		// 业务逻辑改成去查用户的设置然后分别处理，需要同意的走applycontact表
		for _, uid := range memberIDs {
			if err := upsertMemberRel(uid); err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &contactRespond.CreateGroupRespond{
		Uuid:      group.Uuid,
		GroupId:   group.Uuid,
		Name:      group.Name,
		Notice:    group.Notice,
		OwnerId:   group.OwnerId,
		MemberCnt: group.MemberCnt,
		Avatar:    group.Avatar,
		Status:    group.Status,
		CreatedAt: group.CreatedAt.Format(time.RFC3339),
		UpdatedAt: group.UpdatedAt.Format(time.RFC3339),
	}, nil
}

func (s *groupServiceImpl) GetGroupInfo(req contactRequest.GetGroupInfoRequest) (*contactRespond.CreateGroupRespond, error) {
	var group *contactEntity.GroupInfo
	err := s.uow.Transaction(func(_ contactRepository.ContactApplyRepository, _ contactRepository.UserContactRepository, groupRepo contactRepository.GroupInfoRepository) error {
		var err error
		group, err = groupRepo.GetGroupInfoByUUID(req.GroupId)
		return err
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, xerr.New(xerr.NotFound, "群组不存在")
		}
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	return &contactRespond.CreateGroupRespond{
		Uuid:      group.Uuid,
		GroupId:   group.Uuid,
		Name:      group.Name,
		Notice:    group.Notice,
		OwnerId:   group.OwnerId,
		MemberCnt: group.MemberCnt,
		Avatar:    group.Avatar,
		Status:    group.Status,
		CreatedAt: group.CreatedAt.Format(time.RFC3339),
		UpdatedAt: group.UpdatedAt.Format(time.RFC3339),
	}, nil
}

func (s *groupServiceImpl) GetGroupMemberList(req contactRequest.GetGroupMemberListRequest) ([]*contactRespond.GroupMemberRespond, error) {
	var memberRels []contactEntity.UserContact
	var group *contactEntity.GroupInfo

	err := s.uow.Transaction(func(_ contactRepository.ContactApplyRepository, contactRepo contactRepository.UserContactRepository, groupRepo contactRepository.GroupInfoRepository) error {
		var err error
		group, err = groupRepo.GetGroupInfoByUUID(req.GroupId)
		if err != nil {
			return err
		}
		memberRels, err = contactRepo.GetGroupMembers(req.GroupId)
		return err
	})

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, xerr.New(xerr.NotFound, "群组不存在或无成员")
		}
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	memberIDs := make([]string, len(memberRels))
	for i, rel := range memberRels {
		memberIDs[i] = rel.UserId
	}

	userInfos, err := s.userRepo.GetUserContactInfoByUUIDs(memberIDs)
	if err != nil {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	infoMap := make(map[string]*contactEntity.UserContactInfo)
	for i := range userInfos {
		infoMap[userInfos[i].Uuid] = &userInfos[i]
	}

	res := make([]*contactRespond.GroupMemberRespond, 0, len(memberRels))
	for _, rel := range memberRels {
		info, ok := infoMap[rel.UserId]
		if !ok {
			continue
		}

		role := int8(0)
		if rel.UserId == group.OwnerId {
			role = 1
		}

		res = append(res, &contactRespond.GroupMemberRespond{
			UserId:   info.Uuid,
			Username: info.Username,
			Nickname: info.Nickname,
			Avatar:   info.Avatar,
			Gender:   info.Gender,
			Role:     role,
		})
	}
	return res, nil
}

func (s *groupServiceImpl) InviteGroupMembers(req contactRequest.InviteGroupMembersRequest) error {
	if len(req.MemberIds) == 0 {
		return nil
	}

	return s.uow.Transaction(func(_ contactRepository.ContactApplyRepository, contactRepo contactRepository.UserContactRepository, groupRepo contactRepository.GroupInfoRepository) error {
		group, err := groupRepo.GetGroupInfoByUUID(req.GroupId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return xerr.New(xerr.NotFound, "群组不存在")
			}
			return err
		}

		now := time.Now()
		addedCount := 0

		upsertMemberRel := func(userID string) error {
			rel, err := contactRepo.GetUserContactByUserIDAndContactIDAndType(userID, req.GroupId, 1)
			if err == nil {
				if rel.Status == 0 {
					return nil
				}
				rel.ContactType = 1
				rel.Status = 0
				rel.UpdateAt = now
				if err := contactRepo.UpdateUserContact(rel); err != nil {
					return err
				}
				addedCount++
				return nil
			}

			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}

			newRel := &contactEntity.UserContact{
				UserId:      userID,
				ContactId:   req.GroupId,
				ContactType: 1,
				Status:      0,
				CreatedAt:   now,
				UpdateAt:    now,
			}
			if err := contactRepo.CreateUserContact(newRel); err != nil {
				return err
			}
			addedCount++
			return nil
		}

		for _, uid := range req.MemberIds {
			if uid == "" {
				continue
			}
			if err := upsertMemberRel(uid); err != nil {
				zlog.Error(err.Error())
				return xerr.ErrServerError
			}
		}

		if addedCount > 0 {
			allMembers, err := contactRepo.GetGroupMembers(req.GroupId)
			if err == nil {
				var allIDs []string
				for _, m := range allMembers {
					allIDs = append(allIDs, m.UserId)
				}
				membersJSON, _ := json.Marshal(allIDs)
				group.Members = membersJSON
				group.MemberCnt = len(allIDs)
			}

			group.UpdatedAt = now
			if err := groupRepo.UpdateGroupInfo(group); err != nil {
				zlog.Error(err.Error())
				return xerr.ErrServerError
			}
		}

		return nil
	})
}
