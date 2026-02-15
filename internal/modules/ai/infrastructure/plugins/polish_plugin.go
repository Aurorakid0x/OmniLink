package plugins

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// PolishPlugin 文本润色插件
//
// 功能：根据用户输入的句子，提供 2-3 个润色建议
//
// 使用场景：
// - 用户输入完整句子后，提供"更礼貌"、"更简洁"等选项
// - 一键替换输入框内容
type PolishPlugin struct {
	config *PolishConfig
}

// PolishConfig 润色配置
type PolishConfig struct {
	MaxOptions int // 最多返回选项数（默认 3）
	CacheTTL   int // 缓存 TTL（秒，默认 1800）
}

// NewPolishPlugin 创建润色插件
func NewPolishPlugin(config *PolishConfig) *PolishPlugin {
	if config == nil {
		config = &PolishConfig{
			MaxOptions: 3,
			CacheTTL:   1800,
		}
	}
	return &PolishPlugin{config: config}
}

func (p *PolishPlugin) GetServiceType() string {
	return "polish"
}

// BuildPrompt 构建润色 Prompt
//
// Prompt 设计原理：
// 1. 要求 LLM 返回结构化 JSON（而非纯文本）
// 2. 限制选项类型（更礼貌/更简洁/更强硬/更委婉）
// 3. 保证意思不变，只改变语气和风格
func (p *PolishPlugin) BuildPrompt(ctx context.Context, req *PluginRequest) ([]schema.Message, error) {
	systemPrompt := `你是一个智能文本润色助手。分析用户输入的句子，给出 2-3 个润色建议。

输出格式（JSON）：
{
  "polishes": [
    {"label": "更礼貌", "text": "润色后的文本"},
    {"label": "更简洁", "text": "润色后的文本"}
  ]
}

规则：
1. label 必须是："更礼貌"、"更简洁"、"更强硬"、"更委婉"之一
2. 每个选项必须与原句意思一致，只改变语气或风格
3. 如果原句已经很好，可以只返回 1-2 个选项
4. 返回的必须是有效的 JSON 格式`

	userPrompt := fmt.Sprintf("请为以下句子提供润色建议：\n\n%s", req.Input)

	return []schema.Message{
		{Role: schema.System, Content: systemPrompt},
		{Role: schema.User, Content: userPrompt},
	}, nil
}

// ParseResponse 解析润色响应
//
// 设计要点：
// - LLM 应返回 JSON 格式
// - 如果 JSON 解析失败，进行降级处理（返回原始文本）
func (p *PolishPlugin) ParseResponse(ctx context.Context, llmOutput string, req *PluginRequest) (*PluginResponse, error) {
	// 尝试解析 JSON
	var result struct {
		Polishes []struct {
			Label string `json:"label"`
			Text  string `json:"text"`
		} `json:"polishes"`
	}

	// 清理可能的 Markdown 代码块标记
	cleanedOutput := strings.TrimSpace(llmOutput)
	cleanedOutput = strings.TrimPrefix(cleanedOutput, "```json")
	cleanedOutput = strings.TrimPrefix(cleanedOutput, "```")
	cleanedOutput = strings.TrimSuffix(cleanedOutput, "```")
	cleanedOutput = strings.TrimSpace(cleanedOutput)

	if err := json.Unmarshal([]byte(cleanedOutput), &result); err != nil {
		// JSON 解析失败，降级处理
		return &PluginResponse{
			Output: llmOutput, // 直接返回原始输出
			Metadata: map[string]interface{}{
				"parse_error": err.Error(),
				"raw_output":  llmOutput,
			},
		}, nil
	}

	// 限制选项数量
	if len(result.Polishes) > p.config.MaxOptions {
		result.Polishes = result.Polishes[:p.config.MaxOptions]
	}

	// 重新序列化为 JSON
	outputJSON, _ := json.Marshal(result)

	return &PluginResponse{
		Output: string(outputJSON),
		Metadata: map[string]interface{}{
			"options_count": len(result.Polishes),
		},
	}, nil
}

func (p *PolishPlugin) Validate(ctx context.Context, req *PluginRequest) error {
	if req.Input == "" {
		return fmt.Errorf("input is required")
	}
	return nil
}

// GetCacheKey 生成缓存 Key
//
// 润色功能只依赖输入文本，不依赖上下文
// 所以缓存 Key 只需要对输入文本做 Hash
func (p *PolishPlugin) GetCacheKey(ctx context.Context, req *PluginRequest) string {
	hash := md5.Sum([]byte(req.Input))
	return fmt.Sprintf("ai:micro:polish:%s", hex.EncodeToString(hash[:]))
}

func (p *PolishPlugin) GetCacheTTL() int {
	return p.config.CacheTTL
}
