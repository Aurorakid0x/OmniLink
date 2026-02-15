package plugins

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// InputPredictionPlugin 智能输入预测插件
//
// 功能：根据用户当前输入和聊天历史，预测后半句
//
// 使用场景：
// - 用户在输入框输入文字时，实时提供补全建议
// - 类似 Gmail 的智能回复功能
type InputPredictionPlugin struct {
	config *InputPredictionConfig
}

// InputPredictionConfig 智能输入配置
type InputPredictionConfig struct {
	ContextMessages int // 上下文消息数（默认 10）
	MaxInputChars   int // 最大输入字符（默认 500）
	CacheTTL        int // 缓存 TTL（秒，默认 300）
}

// NewInputPredictionPlugin 创建智能输入插件
//
// 参数：
//   - config: 配置（传 nil 使用默认配置）
//
// 返回值：
//   - *InputPredictionPlugin: 插件实例
func NewInputPredictionPlugin(config *InputPredictionConfig) *InputPredictionPlugin {
	// 使用默认配置
	if config == nil {
		config = &InputPredictionConfig{
			ContextMessages: 10,
			MaxInputChars:   500,
			CacheTTL:        300,
		}
	}
	return &InputPredictionPlugin{config: config}
}

// GetServiceType 实现 MicroservicePlugin 接口
func (p *InputPredictionPlugin) GetServiceType() string {
	return "input_prediction"
}

// BuildPrompt 构建智能输入的 Prompt
//
// Prompt 设计原理：
// 1. System Message：定义 AI 的角色和规则
// 2. Context：提供聊天历史，帮助 AI 理解语境
// 3. User Message：明确要求 AI 预测后半句
//
// 注意事项：
// - 只返回补全部分，不重复用户已输入内容
// - 预测内容要简短（< 20 字）
// - 如果无法预测，返回空字符串
func (p *InputPredictionPlugin) BuildPrompt(ctx context.Context, req *PluginRequest) ([]schema.Message, error) {
	// 1. 提取上下文消息
	var contextMsgs []map[string]string
	if msgs, ok := req.Context["messages"].([]interface{}); ok {
		for _, msg := range msgs {
			if m, ok := msg.(map[string]interface{}); ok {
				contextMsgs = append(contextMsgs, map[string]string{
					"role":    fmt.Sprintf("%v", m["role"]),
					"content": fmt.Sprintf("%v", m["content"]),
				})
			}
		}
	}

	// 2. 限制上下文数量（避免 Prompt 过长）
	if len(contextMsgs) > p.config.ContextMessages {
		contextMsgs = contextMsgs[len(contextMsgs)-p.config.ContextMessages:]
	}

	// 3. 构建 System Prompt
	//
	// 设计要点：
	// - 明确 AI 的角色（智能输入助手）
	// - 定义输出规则（简短、不重复、符合语境）
	// - 提供失败处理（无法预测时返回空）
	systemPrompt := `你是一个智能输入助手。根据用户当前输入和聊天历史，预测用户想说的后半句。

规则：
1. 预测内容要简短（< 20 字）
2. 符合聊天语境和用户语气
3. 只返回补全部分，不要重复用户已输入的内容
4. 如果无法预测，返回空字符串`

	// 4. 构建上下文字符串
	contextStr := ""
	for _, msg := range contextMsgs {
		contextStr += fmt.Sprintf("[%s]: %s\n", msg["role"], msg["content"])
	}

	// 5. 构建 User Message
	userPrompt := fmt.Sprintf(`聊天历史：
%s

用户当前输入：%s

请预测后半句（只返回补全部分）：`, contextStr, req.Input)

	// 6. 返回 Eino 标准消息格式
	return []schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userPrompt},
	}, nil
}

// ParseResponse 解析 LLM 响应
//
// 智能输入的响应很简单，直接返回 LLM 输出即可
func (p *InputPredictionPlugin) ParseResponse(ctx context.Context, llmOutput string, req *PluginRequest) (*PluginResponse, error) {
	// 去除前后空格
	prediction := strings.TrimSpace(llmOutput)

	return &PluginResponse{
		Output:     prediction,
		CacheHit:   false, // 由 Pipeline 层填充
		TokensUsed: 0,     // 由 Pipeline 层填充
	}, nil
}

// Validate 验证请求参数
func (p *InputPredictionPlugin) Validate(ctx context.Context, req *PluginRequest) error {
	// 1. 检查输入是否为空
	if req.Input == "" {
		return fmt.Errorf("input is required")
	}

	// 2. 检查输入长度
	if len(req.Input) > p.config.MaxInputChars {
		return fmt.Errorf("input too long (max %d chars)", p.config.MaxInputChars)
	}

	return nil
}

// GetCacheKey 生成缓存 Key
//
// 设计原理：
// - 对 "输入 + 上下文" 生成 MD5 Hash
// - 相同输入和上下文生成相同 Key，实现缓存复用
//
// Key 格式：ai:micro:input:{user_id}:{hash}
func (p *InputPredictionPlugin) GetCacheKey(ctx context.Context, req *PluginRequest) string {
	// 1. 拼接输入和上下文
	data := fmt.Sprintf("%s|%v", req.Input, req.Context["messages"])

	// 2. 计算 MD5 Hash
	hash := md5.Sum([]byte(data))
	hashStr := hex.EncodeToString(hash[:])

	// 3. 返回缓存 Key
	return fmt.Sprintf("ai:micro:input:%s:%s", req.TenantUserID, hashStr)
}

// GetCacheTTL 返回缓存 TTL
func (p *InputPredictionPlugin) GetCacheTTL() int {
	return p.config.CacheTTL
}
