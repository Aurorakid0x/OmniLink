package reader

import (
	"context"
	"fmt"
	"strings"

	contactEntity "OmniLink/internal/modules/contact/domain/entity"
	contactRepository "OmniLink/internal/modules/contact/domain/repository"
	userRepository "OmniLink/internal/modules/user/domain/repository"
)

type ContactProfileDoc struct {
	ContactID string
	Content   string
}

type ContactProfileReader struct {
	contactRepo contactRepository.UserContactRepository
	userRepo    userRepository.UserInfoRepository
}

func NewContactProfileReader(contactRepo contactRepository.UserContactRepository, userRepo userRepository.UserInfoRepository) *ContactProfileReader {
	return &ContactProfileReader{contactRepo: contactRepo, userRepo: userRepo}
}

func (r *ContactProfileReader) ReadContactProfile(ctx context.Context, tenantUserID, contactID string) (string, error) {
	_ = ctx
	if r == nil || r.contactRepo == nil {
		return "", fmt.Errorf("contact repo is nil")
	}
	uid := strings.TrimSpace(tenantUserID)
	cid := strings.TrimSpace(contactID)
	if uid == "" {
		return "", fmt.Errorf("missing tenant_user_id")
	}
	if cid == "" {
		return "", fmt.Errorf("missing contact_id")
	}

	rel, err := r.contactRepo.GetUserContactByUserIDAndContactIDAndType(uid, cid, 0)
	if err != nil {
		return "", err
	}
	if rel == nil || rel.Status != 0 {
		return "", nil
	}

	nickname := ""
	signature := ""
	birthday := ""

	if r.userRepo != nil {
		infos, err := r.userRepo.GetUserContactInfoByUUIDs([]string{cid})
		if err != nil {
			return "", err
		}
		if len(infos) > 0 {
			info := infos[0]
			nickname = strings.TrimSpace(info.Nickname)
			if nickname == "" {
				nickname = strings.TrimSpace(info.Username)
			}
			signature = strings.TrimSpace(info.Signature)
			birthday = formatBirthday(strings.TrimSpace(info.Birthday))
		}
	}

	var b strings.Builder
	b.WriteString("好友详情：")
	if nickname != "" {
		b.WriteString(nickname)
		b.WriteString("，")
	}
	b.WriteString("UUID：")
	b.WriteString(cid)
	if signature != "" {
		b.WriteString("。个性签名：")
		b.WriteString(signature)
	}
	if birthday != "" {
		b.WriteString("。生日：")
		b.WriteString(birthday)
	}
	b.WriteString("。")

	content := strings.TrimSpace(b.String())
	if content == "" {
		return "", nil
	}
	return content, nil
}

func (r *ContactProfileReader) ListContactProfiles(ctx context.Context, tenantUserID string) ([]ContactProfileDoc, error) {
	_ = ctx
	if r == nil || r.contactRepo == nil {
		return nil, fmt.Errorf("contact repo is nil")
	}
	uid := strings.TrimSpace(tenantUserID)
	if uid == "" {
		return nil, fmt.Errorf("missing tenant_user_id")
	}

	contacts, err := r.contactRepo.ListContactsWithInfo(uid)
	if err != nil {
		return nil, err
	}
	if len(contacts) == 0 {
		return []ContactProfileDoc{}, nil
	}

	active := make([]contactEntity.ContactWithUserInfo, 0, len(contacts))
	ids := make([]string, 0, len(contacts))
	seen := map[string]struct{}{}
	for _, c := range contacts {
		if c.ContactType != 0 {
			continue
		}
		if c.Status != 0 {
			continue
		}
		cid := strings.TrimSpace(c.ContactId)
		if cid == "" {
			continue
		}
		active = append(active, c)
		if _, ok := seen[cid]; !ok {
			seen[cid] = struct{}{}
			ids = append(ids, cid)
		}
	}
	if len(active) == 0 {
		return []ContactProfileDoc{}, nil
	}

	infoMap := map[string]contactEntity.UserContactInfo{}
	if r.userRepo != nil && len(ids) > 0 {
		infos, err := r.userRepo.GetUserContactInfoByUUIDs(ids)
		if err != nil {
			return nil, err
		}
		for i := range infos {
			infoMap[strings.TrimSpace(infos[i].Uuid)] = infos[i]
		}
	}

	out := make([]ContactProfileDoc, 0, len(active))
	for _, c := range active {
		cid := strings.TrimSpace(c.ContactId)
		if cid == "" {
			continue
		}

		nickname := strings.TrimSpace(c.Nickname)
		signature := strings.TrimSpace(c.Signature)
		birthday := ""

		if info, ok := infoMap[cid]; ok {
			if strings.TrimSpace(info.Nickname) != "" {
				nickname = strings.TrimSpace(info.Nickname)
			}
			if strings.TrimSpace(info.Signature) != "" {
				signature = strings.TrimSpace(info.Signature)
			}
			birthday = formatBirthday(strings.TrimSpace(info.Birthday))
		}

		var b strings.Builder
		b.WriteString("好友详情：")
		if nickname != "" {
			b.WriteString(nickname)
			b.WriteString("，")
		}
		b.WriteString("UUID：")
		b.WriteString(cid)
		if signature != "" {
			b.WriteString("。个性签名：")
			b.WriteString(signature)
		}
		if birthday != "" {
			b.WriteString("。生日：")
			b.WriteString(birthday)
		}
		b.WriteString("。")

		content := strings.TrimSpace(b.String())
		if content == "" {
			continue
		}
		out = append(out, ContactProfileDoc{ContactID: cid, Content: content})
	}

	return out, nil
}
