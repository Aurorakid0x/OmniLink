package util

import (
	"crypto/rand"
	"strings"

	"github.com/google/uuid"
)

func GenerateUUID() string {
	return uuid.New().String()
}

func GenerateShortUUID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}

func GenerateUserID() string {
	return GenerateID("U")
}

func GenerateGroupID() string {
	return GenerateID("G")
}

func GenerateSessionID() string {
	return GenerateID("S")
}

func GenerateMessageID() string {
	return GenerateID("M")
}

func GenerateApplyID() string {
	return GenerateID("A")
}

func GenerateID(prefix string) string {
	return GenerateIDWithLen(prefix, 11)
}

func GenerateIDWithLen(prefix string, suffixLen int) string {
	if suffixLen <= 0 {
		suffixLen = 11
	}
	return prefix + randomBase62(suffixLen)
}

func randomBase62(n int) string {
	const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	if n <= 0 {
		return ""
	}
	out := make([]byte, 0, n)
	buf := make([]byte, n)
	for len(out) < n {
		_, err := rand.Read(buf)
		if err != nil {
			return GenerateShortUUID()[:n]
		}
		for _, b := range buf {
			if len(out) >= n {
				break
			}
			if b >= 248 {
				continue
			}
			out = append(out, alphabet[int(b)%62])
		}
	}
	return string(out)
}
