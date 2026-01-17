package reader

import (
	"context"
	"fmt"
	"strings"

	contactRepository "OmniLink/internal/modules/contact/domain/repository"
	userEntity "OmniLink/internal/modules/user/domain/entity"
	userRepository "OmniLink/internal/modules/user/domain/repository"
)

type GroupProfileDoc struct {
	GroupID   string
	Content   string
	GroupName string
}

type GroupProfileReader struct {
	groupRepo   contactRepository.GroupInfoRepository
	contactRepo contactRepository.UserContactRepository
	userRepo    userRepository.UserInfoRepository
}

func NewGroupProfileReader(groupRepo contactRepository.GroupInfoRepository, contactRepo contactRepository.UserContactRepository, userRepo userRepository.UserInfoRepository) *GroupProfileReader {
	return &GroupProfileReader{groupRepo: groupRepo, contactRepo: contactRepo, userRepo: userRepo}
}

func (r *GroupProfileReader) ListGroupProfiles(ctx context.Context, tenantUserID string) ([]GroupProfileDoc, error) {
	_ = ctx
	if r == nil || r.groupRepo == nil || r.contactRepo == nil {
		return nil, fmt.Errorf("group/contact repo is nil")
	}
	uid := strings.TrimSpace(tenantUserID)
	if uid == "" {
		return nil, fmt.Errorf("missing tenant_user_id")
	}

	groups, err := r.groupRepo.ListJoinedGroups(uid)
	if err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return []GroupProfileDoc{}, nil
	}

	ownerIDs := make([]string, 0, len(groups))
	seenOwner := map[string]struct{}{}
	for _, g := range groups {
		oid := strings.TrimSpace(g.OwnerId)
		if oid == "" {
			continue
		}
		if _, ok := seenOwner[oid]; ok {
			continue
		}
		seenOwner[oid] = struct{}{}
		ownerIDs = append(ownerIDs, oid)
	}

	ownerMap := map[string]userEntity.UserBrief{}
	if r.userRepo != nil && len(ownerIDs) > 0 {
		owners, err := r.userRepo.GetUserBriefByUUIDs(ownerIDs)
		if err != nil {
			return nil, err
		}
		for i := range owners {
			ownerMap[strings.TrimSpace(owners[i].Uuid)] = owners[i]
		}
	}

	out := make([]GroupProfileDoc, 0, len(groups))
	for _, g := range groups {
		gid := strings.TrimSpace(g.Uuid)
		if gid == "" {
			continue
		}

		members, err := r.contactRepo.GetGroupMembersWithInfo(gid)
		if err != nil {
			continue
		}

		ownerName := strings.TrimSpace(g.OwnerId)
		if ob, ok := ownerMap[strings.TrimSpace(g.OwnerId)]; ok {
			if strings.TrimSpace(ob.Nickname) != "" {
				ownerName = strings.TrimSpace(ob.Nickname)
			}
		}

		var b strings.Builder
		b.WriteString("群组档案：‘")
		b.WriteString(strings.TrimSpace(g.Name))
		b.WriteString("’（ID: ")
		b.WriteString(gid)
		b.WriteString("）。")

		notice := strings.TrimSpace(g.Notice)
		if notice != "" {
			b.WriteString("群公告：‘")
			b.WriteString(notice)
			b.WriteString("’。")
		}

		if ownerName != "" {
			b.WriteString("群主：")
			b.WriteString(ownerName)
			b.WriteString("。")
		}

		if g.MemberCnt > 0 {
			b.WriteString("包含成员 ")
			b.WriteString(fmt.Sprintf("%d", g.MemberCnt))
			b.WriteString(" 人。")
		}

		if len(members) > 0 {
			b.WriteString("成员包括：")
			limit := 30
			if len(members) < limit {
				limit = len(members)
			}
			for i := 0; i < limit; i++ {
				m := members[i]
				name := strings.TrimSpace(m.Nickname)
				if name == "" {
					name = strings.TrimSpace(m.UserId)
				}
				if name == "" {
					continue
				}
				if i > 0 {
					b.WriteString("；")
				}
				b.WriteString(name)

				sig := strings.TrimSpace(m.Signature)
				if sig != "" {
					b.WriteString("（签名：")
					b.WriteString(sig)
					b.WriteString("）")
				}
			}
			if len(members) > limit {
				b.WriteString("等")
			}
			b.WriteString("。")
		}

		content := strings.TrimSpace(b.String())
		if content == "" {
			continue
		}
		out = append(out, GroupProfileDoc{GroupID: gid, GroupName: strings.TrimSpace(g.Name), Content: content})
	}

	return out, nil
}
