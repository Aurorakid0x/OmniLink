package plugins

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// DigestPlugin 消息摘要插件
//
// 功能：分析群聊消息，生成摘要
//
// 使用场景：
// - 群聊未读消息 > 50 条时，提供摘要
// - 帮助用户快速了解错过的讨论
type DigestPlugin struct {
	config *DigestConfig
}

// DigestConfig 摘要配置
type DigestConfig struct {
	MaxMessages int // 最多处理消息数（默认 200）
	CacheTTL    int // 缓存 TTL（秒，默认 600）
}

// NewDigestPlugin 创建摘要插件
func NewDigestPlugin(config *DigestConfig) *DigestPlugin {
	if config == nil {
		config = &DigestConfig{
			MaxMessages: 200,
			CacheTTL:    600,
		}
	}
	return &DigestPlugin{config: config}
}

func (p *DigestPlugin) GetServiceType() string {
	return "digest"
}

// BuildPrompt 构建摘要 Prompt
//
// Prompt 设计原理：
// 1. 要求 LLM 总结主要话题和结论
// 2. 提取待办事项和 @提及
// 3. 使用 Markdown 格式（便于前端渲染）
func (p *DigestPlugin) BuildPrompt(ctx context.Context, req *PluginRequest) ([]schema.Message, error) {
	// 提取消息列表
	var messages []map[string]string
	if msgs, ok := req.Context["messages"].([]interface{}); ok {
		for _, msg := range msgs {
			if m, ok := msg.(map[string]interface{}); ok {
				messages = append(messages, map[string]string{
					"sender":  fmt.Sprintf("%v", m["sender"]),
					"content": fmt.Sprintf("%v", m["content"]),
				})
			}
		}
	}

	// 限制消息数量（避免超过 LLM 窗口）
	if len(messages) > p.config.MaxMessages {
		messages = messages[len(messages)-p.config.MaxMessages:]
	}

	systemPrompt := `你是一个智能群聊摘要助手。分析群聊消息，总结主要话题和关键信息。

输出格式（Markdown）：
### 主要话题
1. 话题1（参与人：@张三、@李四）
2. 话题2

### 重要结论
- 结论1
- 结论2

### 待办事项
- [ ] @张三 需要提交代码（截止时间：明天）

规则：
1. 只提取重要信息，忽略闲聊
2. 提及人名时使用 @
3. 按重要性排序
4. 如果没有待办事项，可以省略该部分`

	// 构建消息文本
	var messageTexts []string
	for _, msg := range messages {
		messageTexts = append(messageTexts, fmt.Sprintf("[%s]: %s", msg["sender"], msg["content"]))
	}

	userPrompt := fmt.Sprintf(`以下是群聊消息（共 %d 条）：

%s

请生成摘要：`, len(messages), strings.Join(messageTexts, "\n"))

	return []schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userPrompt},
	}, nil
}

// ParseResponse 解析摘要响应
//
// 摘要直接返回 Markdown 文本，无需特殊解析
func (p *DigestPlugin) ParseResponse(ctx context.Context, llmOutput string, req *PluginRequest) (*PluginResponse, error) {
	return &PluginResponse{
		Output: llmOutput,
		Metadata: map[string]interface{}{
			"format": "markdown",
		},
	}, nil
}

func (p *DigestPlugin) Validate(ctx context.Context, req *PluginRequest) error {
	if req.Context["messages"] == nil {
		return fmt.Errorf("messages context is required")
	}
	return nil
}

// GetCacheKey 生成缓存 Key
//
// 设计要点：
// - 对 "群组ID + 消息列表" 生成 Hash
// - 相同消息范围生成相同 Key
func (p *DigestPlugin) GetCacheKey(ctx context.Context, req *PluginRequest) string {
	groupID := ""
	if gid, ok := req.Context["group_id"].(string); ok {
		groupID = gid
	}

	data := fmt.Sprintf("%s|%v", groupID, req.Context["messages"])
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("ai:micro:digest:%s:%s", groupID, hex.EncodeToString(hash[:]))
}

func (p *DigestPlugin) GetCacheTTL() int {
	return p.config.CacheTTL
}
