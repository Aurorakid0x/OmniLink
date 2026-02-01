package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	aiIngest "OmniLink/internal/modules/ai/application/service"
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
	LeaveGroup(req contactRequest.LeaveGroupRequest) error
	DismissGroup(req contactRequest.DismissGroupRequest) error
}

type groupServiceImpl struct {
	userRepo userRepository.UserInfoRepository
	uow      contactRepository.ContactUnitOfWork
	aiIngest aiIngest.AsyncIngestService
}

func NewGroupService(
	_ contactRepository.UserContactRepository,
	_ contactRepository.GroupInfoRepository,
	userRepo userRepository.UserInfoRepository,
	uow contactRepository.ContactUnitOfWork,
	aiIngestSvc aiIngest.AsyncIngestService,
) GroupService {
	return &groupServiceImpl{
		userRepo: userRepo,
		uow:      uow,
		aiIngest: aiIngestSvc,
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
		//TODO：有时间改成用户可以设置自己的入群模式——1.被邀请入群无需同意，2.被邀请入群需要自己同意，
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

	if s.aiIngest != nil {
		for _, uid := range memberIDs {
			_ = s.aiIngest.EnqueueGroupProfile(context.Background(), uid, groupID)
		}
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

	var updatedMembers []string
	err := s.uow.Transaction(func(_ contactRepository.ContactApplyRepository, contactRepo contactRepository.UserContactRepository, groupRepo contactRepository.GroupInfoRepository) error {
		group, err := groupRepo.GetGroupInfoByUUID(req.GroupId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return xerr.New(xerr.NotFound, "群组不存在")
			}
			return err
		}

		if group.Status != 0 {
			return xerr.New(xerr.Forbidden, "群组已解散或状态异常，无法邀请成员")
		}

		now := time.Now()
		addedCount := 0

		upsertMemberRel := func(userID string) error {
			rel, err := contactRepo.GetUserContactByUserIDAndContactIDAndType(userID, req.GroupId, 1)
			if err == nil {
				if rel.Status == 0 || rel.Status == 5 {
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
				updatedMembers = append([]string(nil), allIDs...)
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
	if err != nil {
		return err
	}

	if s.aiIngest != nil {
		for _, uid := range updatedMembers {
			_ = s.aiIngest.EnqueueGroupProfile(context.Background(), uid, req.GroupId)
		}
	}

	return nil
}

func (s *groupServiceImpl) LeaveGroup(req contactRequest.LeaveGroupRequest) error {
	var remainingMembers []string
	returnErr := s.uow.Transaction(func(_ contactRepository.ContactApplyRepository, contactRepo contactRepository.UserContactRepository, groupRepo contactRepository.GroupInfoRepository) error {
		group, err := groupRepo.GetGroupInfoByUUID(req.GroupId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return xerr.New(xerr.NotFound, "群组不存在")
			}
			return err
		}

		if group.Status != 0 {
			return xerr.New(xerr.Forbidden, "群组已解散或状态异常，无法退群")
		}

		if group.OwnerId == req.OwnerId {
			return xerr.New(xerr.Forbidden, "群主不能退群，请先转让群主或解散群")
		}

		rel, err := contactRepo.GetUserContactByUserIDAndContactIDAndType(req.OwnerId, req.GroupId, 1)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return xerr.New(xerr.BadRequest, "非群成员")
			}
			return err
		}
		if rel.Status != 0 && rel.Status != 5 {
			return xerr.New(xerr.BadRequest, "非群成员")
		}

		// Update status to 6 (Quit)
		rel.Status = 6
		rel.UpdateAt = time.Now()
		if err := contactRepo.UpdateUserContact(rel); err != nil {
			return err
		}

		// Update group info members
		var members []string
		if err := json.Unmarshal(group.Members, &members); err != nil {
			return err
		}

		newMembers := make([]string, 0, len(members))
		for _, m := range members {
			if m != req.OwnerId {
				newMembers = append(newMembers, m)
			}
		}

		membersJSON, _ := json.Marshal(newMembers)
		group.Members = membersJSON
		group.MemberCnt = len(newMembers)
		group.UpdatedAt = time.Now()
		remainingMembers = append([]string(nil), newMembers...)

		return groupRepo.UpdateGroupInfo(group)
	})
	if returnErr != nil {
		return returnErr
	}

	if s.aiIngest != nil {
		_ = s.aiIngest.EnqueueGroupProfile(context.Background(), req.OwnerId, req.GroupId)
		for _, uid := range remainingMembers {
			_ = s.aiIngest.EnqueueGroupProfile(context.Background(), uid, req.GroupId)
		}
	}

	return nil
}

func (s *groupServiceImpl) DismissGroup(req contactRequest.DismissGroupRequest) error {
	var members []string
	var groupID string

	err := s.uow.Transaction(func(_ contactRepository.ContactApplyRepository, _ contactRepository.UserContactRepository, groupRepo contactRepository.GroupInfoRepository) error {
		group, err := groupRepo.GetGroupInfoByUUID(req.GroupId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return xerr.New(xerr.NotFound, "群组不存在")
			}
			return err
		}

		if group.OwnerId != req.OwnerId {
			return xerr.New(xerr.Forbidden, "非群主无法解散群聊")
		}

		if group.Status == 2 {
			return xerr.New(xerr.Forbidden, "群组已解散，请勿重复操作")
		}
		if group.Status != 0 {
			return xerr.New(xerr.Forbidden, "群组状态异常，无法解散")
		}

		groupID = group.Uuid
		_ = json.Unmarshal(group.Members, &members)

		group.Status = 2
		group.UpdatedAt = time.Now()
		return groupRepo.UpdateGroupInfo(group)
	})
	if err != nil {
		return err
	}

	if groupID != "" {
		go func(gid string) {
			updateAt := time.Now()
			updateErr := s.uow.Transaction(func(_ contactRepository.ContactApplyRepository, contactRepo contactRepository.UserContactRepository, _ contactRepository.GroupInfoRepository) error {
				return contactRepo.UpdateGroupContactsStatus(gid, 8, updateAt)
			})
			if updateErr != nil {
				zlog.Error(updateErr.Error())
			}
		}(groupID)
	}

	if s.aiIngest != nil {
		for _, uid := range members {
			if strings.TrimSpace(uid) == "" {
				continue
			}
			_ = s.aiIngest.EnqueueGroupProfile(context.Background(), uid, groupID)
		}
	}

	return nil
}
