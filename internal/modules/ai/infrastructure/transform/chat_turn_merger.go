package transform

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"OmniLink/internal/modules/chat/domain/entity"
)

// ChatTurnMerger 用于将消息聚合为连贯的“对话片段”
type ChatTurnMerger struct {
	TimeWindow time.Duration
}

// NewChatTurnMerger 创建一个默认时间窗口为 5 分钟的聚合器
func NewChatTurnMerger() *ChatTurnMerger {
	return &ChatTurnMerger{
		TimeWindow: 5 * time.Minute,
	}
}

// Merge 将消息聚合为多个对话片段。
// 它会先按 SessionId 分组，再在每组内按 TimeWindow 合并相邻消息。
func (m *ChatTurnMerger) Merge(messages []entity.Message) []string {
	if len(messages) == 0 {
		return []string{}
	}

	// 1) 按 SessionId 分组（防御性：避免混入不同会话的消息）
	sessions := make(map[string][]entity.Message)
	for _, msg := range messages {
		sessions[msg.SessionId] = append(sessions[msg.SessionId], msg)
	}

	var result []string

	// 2) 逐个会话处理
	for _, sessionMsgs := range sessions {
		if len(sessionMsgs) == 0 {
			continue
		}

		// 按创建时间升序排序（从旧到新），保证对话是时间顺序
		sort.Slice(sessionMsgs, func(i, j int) bool {
			return sessionMsgs[i].CreatedAt.Before(sessionMsgs[j].CreatedAt)
		})

		var currentSegment strings.Builder
		var lastTime time.Time
		isFirst := true

		for _, msg := range sessionMsgs {
			// 跳过空内容
			content := strings.TrimSpace(msg.Content)
			if content == "" {
				continue
			}

			// 判断是否需要开启新的对话片段
			if !isFirst {
				if msg.CreatedAt.Sub(lastTime) > m.TimeWindow {
					// 超过时间窗口：把当前片段收口，开始新片段
					if currentSegment.Len() > 0 {
						result = append(result, currentSegment.String())
						currentSegment.Reset()
					}
				} else {
					// 在同一个时间窗口内：用换行分隔多条消息
					currentSegment.WriteString("\n")
				}
			}

			// 格式："发送者(时:分:秒): 内容"
			// 例："张三(10:00:00): 你好"
			timeStr := msg.CreatedAt.Format("15:04:05")
			line := fmt.Sprintf("%s(%s): %s", msg.SendName, timeStr, content)
			currentSegment.WriteString(line)

			lastTime = msg.CreatedAt
			isFirst = false
		}

		// 收口最后一个片段
		if currentSegment.Len() > 0 {
			result = append(result, currentSegment.String())
		}
	}

	return result
}