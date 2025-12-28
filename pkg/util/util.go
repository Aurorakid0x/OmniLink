package util

import (
	"strings"

	"github.com/google/uuid"
)

// GenerateUUID 生成一个标准的 UUID (v4)
func GenerateUUID() string {
	return uuid.New().String()
}

// GenerateShortUUID 生成一个不带中划线的短 UUID
func GenerateShortUUID() string {
	return strings.ReplaceAll(uuid.New().String(), "-", "")
}
