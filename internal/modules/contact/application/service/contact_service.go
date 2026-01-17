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
	userRepository "OmniLink/internal/modules/user/domain/repository"
	"OmniLink/pkg/util"
	"OmniLink/pkg/xerr"
	"OmniLink/pkg/zlog"

	"gorm.io/gorm"
)

type ContactService interface {
	GetUserList(req contactRequest.GetUserListRequest) ([]contactRespond.UserListItem, error)
	GetContactInfo(req contactRequest.GetContactInfoRequest) (*contactRespond.GetContactInfoRespond, error)
	ApplyContact(req contactRequest.ApplyContactRequest) (*contactRespond.ApplyContactRespond, error)
	GetNewContactList(req contactRequest.GetNewContactListRequest) ([]contactRespond.NewContactApplyItem, error)
	PassContactApply(req contactRequest.PassContactApplyRequest) error
	RefuseContactApply(req contactRequest.RefuseContactApplyRequest) error
	LoadMyJoinedGroup(req contactRequest.LoadMyJoinedGroupRequest) ([]contactRespond.JoinedGroupItem, error)
}

type contactServiceImpl struct {
	contactRepo contactRepository.UserContactRepository
	applyRepo   contactRepository.ContactApplyRepository
	userRepo    userRepository.UserInfoRepository
	uow         contactRepository.ContactUnitOfWork
	aiIngest    aiIngest.AsyncIngestService
}

func NewContactService(contactRepo contactRepository.UserContactRepository, applyRepo contactRepository.ContactApplyRepository, userRepo userRepository.UserInfoRepository, uow contactRepository.ContactUnitOfWork, aiIngestSvc aiIngest.AsyncIngestService) ContactService {
	return &contactServiceImpl{
		contactRepo: contactRepo,
		applyRepo:   applyRepo,
		userRepo:    userRepo,
		uow:         uow,
		aiIngest:    aiIngestSvc,
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

func (s *contactServiceImpl) LoadMyJoinedGroup(req contactRequest.LoadMyJoinedGroupRequest) ([]contactRespond.JoinedGroupItem, error) {
	if req.OwnerId == "" {
		return nil, xerr.New(xerr.BadRequest, xerr.ErrParam.Message)
	}

	contacts, err := s.contactRepo.GetUserContactsByUserID(req.OwnerId)
	if err != nil {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	var groupIDs []string
	for _, c := range contacts {
		if c.ContactType == 1 && (c.Status == 0 || c.Status == 5) {
			groupIDs = append(groupIDs, c.ContactId)
		}
	}

	if len(groupIDs) == 0 {
		return []contactRespond.JoinedGroupItem{}, nil
	}

	var items []contactRespond.JoinedGroupItem
	err = s.uow.Transaction(func(_ contactRepository.ContactApplyRepository, _ contactRepository.UserContactRepository, groupRepo contactRepository.GroupInfoRepository) error {
		for _, gid := range groupIDs {
			info, err := groupRepo.GetGroupInfoByUUID(gid)
			if err != nil {
				// 忽略单个查询错误
				continue
			}
			items = append(items, contactRespond.JoinedGroupItem{
				GroupId:   info.Uuid,
				GroupName: info.Name,
				Avatar:    info.Avatar,
			})
		}
		return nil
	})

	if err != nil {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	return items, nil
}

func (s *contactServiceImpl) PassContactApply(req contactRequest.PassContactApplyRequest) error {
	if req.OwnerId == "" || req.ApplyId == "" {
		return xerr.New(xerr.BadRequest, xerr.ErrParam.Message)
	}

	now := time.Now()
	var friendA string
	var friendB string
	var groupID string
	var groupMembers []string
	var groupApplicant string

	err := s.uow.Transaction(func(applyRepo contactRepository.ContactApplyRepository, contactRepo contactRepository.UserContactRepository, groupRepo contactRepository.GroupInfoRepository) error {
		apply, err := applyRepo.GetContactApplyByUUIDForUpdate(req.ApplyId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return xerr.New(xerr.NotFound, "申请不存在")
			}
			zlog.Error(err.Error())
			return xerr.ErrServerError
		}

		if apply.Status == 1 {
			return nil
		}
		if apply.Status == 3 {
			return xerr.New(xerr.Forbidden, "该申请已被拉黑")
		}

		// 好友申请处理
		if apply.ContactType == 0 {
			if apply.ContactId != req.OwnerId {
				return xerr.New(xerr.Forbidden, "无权操作该申请")
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
			friendA = apply.UserId
			friendB = apply.ContactId
			return nil
		}

		// 群组申请处理
		if apply.ContactType == 1 {
			group, err := groupRepo.GetGroupInfoByUUID(apply.ContactId)
			if err != nil {
				return err
			}
			if group.Status != 0 {
				return xerr.New(xerr.Forbidden, "群组状态异常")
			}
			if group.OwnerId != req.OwnerId {
				return xerr.New(xerr.Forbidden, "只有群主可以审批入群申请")
			}

			apply.Status = 1
			if err := applyRepo.UpdateContactApply(apply); err != nil {
				return err
			}

			// Upsert 群成员关系
			rel, err := contactRepo.GetUserContactByUserIDAndContactIDAndType(apply.UserId, apply.ContactId, 1)
			if err == nil {
				if rel.Status != 0 {
					rel.Status = 0
					rel.UpdateAt = now
					if err := contactRepo.UpdateUserContact(rel); err != nil {
						return err
					}
				}
			} else if errors.Is(err, gorm.ErrRecordNotFound) {
				newRel := &contactEntity.UserContact{
					UserId:      apply.UserId,
					ContactId:   apply.ContactId,
					ContactType: 1,
					Status:      0,
					CreatedAt:   now,
					UpdateAt:    now,
				}
				if err := contactRepo.CreateUserContact(newRel); err != nil {
					return err
				}
			} else {
				return err
			}

			// 更新群成员信息
			allMembers, err := contactRepo.GetGroupMembers(apply.ContactId)
			if err == nil {
				var allIDs []string
				for _, m := range allMembers {
					allIDs = append(allIDs, m.UserId)
				}
				groupMembers = append([]string(nil), allIDs...)
				membersJSON, _ := json.Marshal(allIDs)
				group.Members = membersJSON
				group.MemberCnt = len(allIDs)
				group.UpdatedAt = now
				if err := groupRepo.UpdateGroupInfo(group); err != nil {
					return err
				}
			}

			groupID = apply.ContactId
			groupApplicant = apply.UserId
			return nil
		}

		return xerr.New(xerr.BadRequest, "不支持的申请类型")
	})
	if err != nil {
		return err
	}

	if s.aiIngest != nil {
		if friendA != "" && friendB != "" {
			_ = s.aiIngest.EnqueueContactProfile(context.Background(), friendA, friendB)
			_ = s.aiIngest.EnqueueContactProfile(context.Background(), friendB, friendA)
		}
		if groupID != "" {
			members := groupMembers
			if len(members) == 0 && groupApplicant != "" {
				members = []string{groupApplicant}
			}
			for _, uid := range members {
				_ = s.aiIngest.EnqueueGroupProfile(context.Background(), uid, groupID)
			}
		}
	}

	return nil
}

func (s *contactServiceImpl) RefuseContactApply(req contactRequest.RefuseContactApplyRequest) error {
	if req.OwnerId == "" || req.ApplyId == "" {
		return xerr.New(xerr.BadRequest, xerr.ErrParam.Message)
	}

	return s.uow.Transaction(func(applyRepo contactRepository.ContactApplyRepository, _ contactRepository.UserContactRepository, groupRepo contactRepository.GroupInfoRepository) error {
		apply, err := applyRepo.GetContactApplyByUUIDForUpdate(req.ApplyId)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return xerr.New(xerr.NotFound, "申请不存在")
			}
			zlog.Error(err.Error())
			return xerr.ErrServerError
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

		switch apply.ContactType {
		case 0:
			if apply.ContactId != req.OwnerId {
				return xerr.New(xerr.Forbidden, "无权操作该申请")
			}
		case 1:
			group, err := groupRepo.GetGroupInfoByUUID(apply.ContactId)
			if err != nil {
				return err
			}
			if group.OwnerId != req.OwnerId {
				return xerr.New(xerr.Forbidden, "只有群主可以审批")
			}
		default:
			return xerr.New(xerr.BadRequest, "不支持的申请类型")
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

	var applyID string
	err := s.uow.Transaction(func(applyRepo contactRepository.ContactApplyRepository, contactRepo contactRepository.UserContactRepository, groupRepo contactRepository.GroupInfoRepository) error {
		if contactType == 0 {
			briefs, err := s.userRepo.GetUserBriefByUUIDs([]string{req.ContactId})
			if err != nil {
				zlog.Error(err.Error())
				return xerr.ErrServerError
			}
			if len(briefs) == 0 {
				return xerr.New(xerr.NotFound, "用户不存在")
			}
			if briefs[0].Status != 0 {
				return xerr.New(xerr.Forbidden, "用户不可用")
			}
		} else {
			// 群组检查
			group, err := groupRepo.GetGroupInfoByUUID(req.ContactId)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return xerr.New(xerr.NotFound, "群组不存在")
				}
				zlog.Error(err.Error())
				return xerr.ErrServerError
			}
			if group.Status != 0 {
				return xerr.New(xerr.Forbidden, "群组状态异常")
			}
			// 检查是否已经是成员
			rel, err := contactRepo.GetUserContactByUserIDAndContactIDAndType(req.OwnerId, req.ContactId, 1)
			if err == nil && (rel.Status == 0 || rel.Status == 5) {
				return xerr.New(xerr.BadRequest, "已在群聊中")
			}
		}

		// 检查关系状态 (仅针对好友，群组已经在上面查了成员关系)
		if contactType == 0 {
			if rel, err := contactRepo.GetUserContactByUserIDAndContactID(req.OwnerId, req.ContactId); err == nil {
				if rel.ContactType == contactType {
					switch rel.Status {
					case 0:
						return xerr.New(xerr.BadRequest, "已是好友")
					case 1:
						return xerr.New(xerr.BadRequest, "您已将对方拉黑")
					case 2:
						return xerr.New(xerr.Forbidden, "对方已将您拉黑")
					}
				}
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				zlog.Error(err.Error())
				return xerr.ErrServerError
			}
		}

		now := time.Now()
		apply, err := applyRepo.GetContactApplyByUserIDAndContactID(req.OwnerId, req.ContactId, contactType)
		if err == nil {
			apply.Status = 0
			apply.Message = req.Message
			if apply.Message == "" && contactType == 1 {
				apply.Message = "申请加入群聊"
			}
			apply.LastApplyAt = now
			if apply.Uuid == "" {
				apply.Uuid = util.GenerateApplyID()
			}
			if err := applyRepo.UpdateContactApply(apply); err != nil {
				zlog.Error(err.Error())
				return xerr.ErrServerError
			}
			applyID = apply.Uuid
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			zlog.Error(err.Error())
			return xerr.ErrServerError
		}

		msg := req.Message
		if msg == "" && contactType == 1 {
			msg = "申请加入群聊"
		}
		newApply := contactEntity.ContactApply{
			Uuid:        util.GenerateApplyID(),
			UserId:      req.OwnerId,
			ContactId:   req.ContactId,
			ContactType: contactType,
			Status:      0,
			Message:     msg,
			LastApplyAt: now,
		}
		if err := applyRepo.CreateContactApply(&newApply); err != nil {
			zlog.Error(err.Error())
			return xerr.ErrServerError
		}
		applyID = newApply.Uuid
		return nil
	})

	if err != nil {
		return nil, err
	}

	return &contactRespond.ApplyContactRespond{ApplyId: applyID}, nil
}

func (s *contactServiceImpl) GetNewContactList(req contactRequest.GetNewContactListRequest) ([]contactRespond.NewContactApplyItem, error) {
	if req.OwnerId == "" {
		return nil, xerr.New(xerr.BadRequest, xerr.ErrParam.Message)
	}

	var allApplies []contactEntity.ContactApply

	err := s.uow.Transaction(func(applyRepo contactRepository.ContactApplyRepository, _ contactRepository.UserContactRepository, groupRepo contactRepository.GroupInfoRepository) error {
		// 1. 获取好友申请
		friendApplies, err := applyRepo.ListPendingAppliesByContactID(req.OwnerId)
		if err != nil {
			return err
		}

		// 2. 获取我拥有的群组
		myGroups, err := groupRepo.ListByOwnerID(req.OwnerId)
		if err != nil {
			return err
		}

		// 3. 获取群组申请
		var groupApplies []contactEntity.ContactApply
		for _, g := range myGroups {
			apps, err := applyRepo.ListPendingAppliesByContactID(g.Uuid)
			if err != nil {
				return err
			}
			groupApplies = append(groupApplies, apps...)
		}

		// 合并
		allApplies = append(friendApplies, groupApplies...)
		return nil
	})

	if err != nil {
		zlog.Error(err.Error())
		return nil, xerr.ErrServerError
	}

	if len(allApplies) == 0 {
		return []contactRespond.NewContactApplyItem{}, nil
	}

	userIDs := make([]string, 0, len(allApplies))
	seen := make(map[string]struct{}, len(allApplies))
	for _, a := range allApplies {
		// 无论是好友申请还是群申请，UserId 都是申请人
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

	out := make([]contactRespond.NewContactApplyItem, 0, len(allApplies))
	for _, a := range allApplies {
		// 过滤非 0 状态 (虽然 ListPending 已经过滤了，但保险起见)
		if a.Status != 0 {
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
		// 可以考虑在 Message 或其他字段标识这是群申请
		if a.ContactType == 1 {
			item.Message = "[申请入群] " + item.Message
		}
		out = append(out, item)
	}

	// 简单的按时间倒序排序
	// ListPendingAppliesByContactID 已经按时间倒序，但合并后顺序可能乱了
	// 这里暂不重新排序，或者假设前端会处理，或者由于只是少量数据，可以接受
	return out, nil
}
