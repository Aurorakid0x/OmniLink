package service

import (
	"context"
	"fmt"
	"time"

	"OmniLink/internal/modules/ai/domain/agent"
	"OmniLink/internal/modules/ai/domain/assistant"
	"OmniLink/internal/modules/ai/domain/rag"
	"OmniLink/internal/modules/ai/domain/repository"
	"OmniLink/pkg/util"
)

// UserLifecycleService 用户生命周期服务（处理用户注册、注销等AI相关初始化）
type UserLifecycleService interface {
	// InitializeUserAIAssistant 用户注册后初始化AI助手（创建全局Agent和系统会话）
	InitializeUserAIAssistant(ctx context.Context, tenantUserID string) error
}

type userLifecycleServiceImpl struct {
	agentRepo   repository.AgentRepository
	sessionRepo repository.AssistantSessionRepository
	ragRepo     repository.RAGRepository
}

// NewUserLifecycleService 创建用户生命周期服务
func NewUserLifecycleService(
	agentRepo repository.AgentRepository,
	sessionRepo repository.AssistantSessionRepository,
	ragRepo repository.RAGRepository,
) UserLifecycleService {
	return &userLifecycleServiceImpl{
		agentRepo:   agentRepo,
		sessionRepo: sessionRepo,
		ragRepo:     ragRepo,
	}
}

func (s *userLifecycleServiceImpl) InitializeUserAIAssistant(ctx context.Context, tenantUserID string) error {
	// 1. 检查是否已初始化（幂等性保证）
	existingAgent, err := s.agentRepo.GetSystemGlobalAgent(ctx, tenantUserID)
	if err != nil {
		return fmt.Errorf("failed to check existing agent: %w", err)
	}
	if existingAgent == nil {
		// 2. 创建全局知识库（如果不存在）
		kb := &rag.AIKnowledgeBase{
			OwnerType: "user", // 归属用户
			OwnerId:   tenantUserID,
			KBType:    agent.KBTypeGlobal,
			Name:      "Global Knowledge Base",
			Status:    rag.CommonStatusEnabled,
		}
		kbID, err := s.ragRepo.EnsureKnowledgeBase(ctx, kb)
		if err != nil {
			return fmt.Errorf("failed to ensure knowledge base: %w", err)
		}

		// 3. 创建系统全局AI助手Agent
		systemPrompt := `### 基础身份
你是由 OmniLink 构建的全局 AI 个人助手。你的核心目标是辅助用户管理社交关系、处理消息并提供智能问答。

### 核心能力与约束
1. **数据严谨性**：
   - 对于用户的私有数据（好友列表、群组信息、聊天记录），**必须** 通过工具调用（Tools）或检索增强生成（RAG）获取，**严禁** 臆造。
   - 若工具或检索未返回结果，请明确告知用户"未找到相关信息"，不要编造假数据。

2. **工具使用策略**：
   - 当用户询问"我有没有好友X"、"发消息给Y"、"最近群里聊了什么"等实时操作类问题时，**优先** 尝试调用对应的 MCP 工具。
   - 若无可用工具，请向用户解释当前能力受限。

3. **回答风格**：
   - 简洁、专业、友好。
   - 涉及敏感隐私（如手机号、详细地址）时，请进行脱敏处理或再次确认。

### 知识库范围
你拥有全局知识库的访问权限，可以回答关于 OmniLink 平台功能、通用百科等问题。

### 扩展能力（预留）
未来你将支持：
- 离线总结：用户登录时自动推送离线期间的重点消息摘要
- 主动通知：定时提醒、日报推送等
- 智能指令：通过 /todo、/remind 等快捷命令快速执行任务`

		newAgent := &agent.AIAgent{
			AgentId:          util.GenerateID("AG"),
			OwnerType:        agent.OwnerTypeUser,
			OwnerId:          tenantUserID,
			Name:             "全局AI助手",
			Description:      "您的专属智能助理，负责消息管理、智能问答和主动通知",
			PersonaPrompt:    "", // 系统助手无需用户自定义人设
			SystemPrompt:     systemPrompt,
			Status:           agent.AgentStatusEnabled,
			KBType:           agent.KBTypeGlobal,
			KBId:             kbID,
			ToolsJson:        "[]", // 预留，后续配置MCP工具
			IsSystemGlobal:   agent.IsSystemGlobalTrue,
			CapabilitiesJson: "{}", // 预留
			ConfigJson:       "{}", // 预留
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
		}

		if err := s.agentRepo.CreateSystemGlobalAgent(ctx, newAgent); err != nil {
			return fmt.Errorf("failed to create system global agent: %w", err)
		}
		existingAgent = newAgent
	}

	// 4. 创建系统级助手会话（若不存在）
	session, err := s.sessionRepo.GetSystemGlobalSession(ctx, tenantUserID)
	if err != nil {
		return fmt.Errorf("failed to get system global session: %w", err)
	}
	if session != nil {
		return nil
	}

	newSession := &assistant.AIAssistantSession{
		SessionId:         util.GenerateID("AS"),
		TenantUserId:      tenantUserID,
		Title:             "🤖 AI助手",
		Status:            assistant.SessionStatusActive,
		AgentId:           existingAgent.AgentId,
		SessionType:       assistant.SessionTypeSystemGlobal,
		IsPinned:          assistant.IsPinnedTrue,
		IsDeletable:       assistant.IsDeletableFalse,
		ContextConfigJson: "{}", // 预留
		MetadataJson:      "{}", // 预留
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	if err := s.sessionRepo.CreateSystemGlobalSession(ctx, newSession); err != nil {
		return fmt.Errorf("failed to create system global session: %w", err)
	}

	return nil
}
