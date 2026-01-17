package reader

import (
	"context"
	"fmt"
	"strings"
	"time"

	userRepository "OmniLink/internal/modules/user/domain/repository"
)

type SelfProfileReader struct {
	userRepo userRepository.UserInfoRepository
}

func NewSelfProfileReader(userRepo userRepository.UserInfoRepository) *SelfProfileReader {
	return &SelfProfileReader{userRepo: userRepo}
}

func (r *SelfProfileReader) ReadProfile(ctx context.Context, tenantUserID string) (string, error) {
	_ = ctx
	if r == nil || r.userRepo == nil {
		return "", fmt.Errorf("user repo is nil")
	}

	uid := strings.TrimSpace(tenantUserID)
	if uid == "" {
		return "", fmt.Errorf("missing tenant_user_id")
	}

	u, err := r.userRepo.GetUserInfoByUUIDWithoutPassword(uid)
	if err != nil {
		return "", err
	}
	if u == nil {
		return "", nil
	}

	var b strings.Builder
	b.WriteString("我的个人档案：")

	writeCNField(&b, "昵称", strings.TrimSpace(u.Nickname))
	writeCNField(&b, "UUID", strings.TrimSpace(u.Uuid))

	bd := formatBirthday(strings.TrimSpace(u.Birthday))
	writeCNField(&b, "生日", bd)

	writeCNField(&b, "个性签名", strings.TrimSpace(u.Signature))
	if !u.CreatedAt.IsZero() {
		writeCNField(&b, "注册时间", u.CreatedAt.Format("2006-01-02"))
	}

	out := strings.TrimSpace(b.String())
	if out == "我的个人档案：" {
		return "", nil
	}
	return out, nil
}

func writeCNField(b *strings.Builder, k, v string) {
	if b == nil {
		return
	}
	k = strings.TrimSpace(k)
	v = strings.TrimSpace(v)
	if k == "" || v == "" {
		return
	}
	if b.Len() > 0 && !strings.HasSuffix(b.String(), "：") {
		b.WriteString("，")
	}
	b.WriteString(k)
	b.WriteString(" ")
	b.WriteString(v)
}

func formatBirthday(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.Contains(s, "-") && len(s) >= 8 {
		return s
	}
	if len(s) == 8 {
		yyyy := s[0:4]
		mm := s[4:6]
		dd := s[6:8]
		if isAllDigits(yyyy) && isAllDigits(mm) && isAllDigits(dd) {
			return yyyy + "-" + mm + "-" + dd
		}
	}
	if t, err := time.Parse("20060102", s); err == nil {
		return t.Format("2006-01-02")
	}
	return s
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
