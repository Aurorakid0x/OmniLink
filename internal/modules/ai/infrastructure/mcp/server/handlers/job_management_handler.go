package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"OmniLink/internal/modules/ai/application/service"
	"OmniLink/internal/modules/ai/domain/repository"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type JobManagementHandler struct {
	jobSvc    service.AIJobService
	agentRepo repository.AgentRepository
}

func NewJobManagementHandler(svc service.AIJobService, agentRepo repository.AgentRepository) *JobManagementHandler {
	return &JobManagementHandler{
		jobSvc:    svc,
		agentRepo: agentRepo,
	}
}

func (h *JobManagementHandler) RegisterTools(s *server.MCPServer) {
	// 1. Tool: manage_ai_job (核心创建/删除工具)
	s.AddTool(mcp.NewTool("manage_ai_job",
		mcp.WithDescription("创建或管理 AI 自动化任务。支持定时(cron)、一次性(once)、事件驱动(event)三种模式。"),
		mcp.WithString("action", mcp.Required(), mcp.Description("操作类型: create | delete")),
		mcp.WithString("trigger_type", mcp.Required(), mcp.Description("触发类型: once | cron | event")),
		mcp.WithString("trigger_value", mcp.Description("触发值: once传ISO时间(2006-01-02T15:04:05Z), cron传表达式(0 8 * * *), event传事件key(user_login)")),
		mcp.WithString("prompt", mcp.Description("任务执行时发送给Agent的指令Prompt")),
		mcp.WithString("agent_id", mcp.Description("执行任务的AgentID (可选，默认使用当前Agent)")),
	), h.handleManageJob)

	// 2. Tool: list_my_agents (辅助工具，查询用户有哪些Agent)
	s.AddTool(mcp.NewTool("list_my_agents",
		mcp.WithDescription("列出当前用户拥有的所有 Agent，用于获取 agent_id"),
	), h.handleListAgents)

	// 3. Tool: list_supported_events (辅助工具，查询支持哪些事件)
	s.AddTool(mcp.NewTool("list_supported_events",
		mcp.WithDescription("列出系统支持的所有触发事件 Key"),
	), h.handleListEvents)
}

func (h *JobManagementHandler) handleManageJob(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		return mcp.NewToolResultError("invalid arguments format"), nil
	}
	action, _ := args["action"].(string)
	triggerType, _ := args["trigger_type"].(string)
	triggerValue, _ := args["trigger_value"].(string)
	prompt, _ := args["prompt"].(string)
	targetAgentID, _ := args["agent_id"].(string)

	// 需要 Pipeline 注入 UserID 和 AgentID
	var userID, currentAgentID string
	if v := ctx.Value("tenant_user_id"); v != nil {
		if value, ok := v.(string); ok {
			userID = value
		}
	}
	if v := ctx.Value("agent_id"); v != nil {
		if value, ok := v.(string); ok {
			currentAgentID = value
		}
	}

	if targetAgentID == "" {
		targetAgentID = currentAgentID
	}

	if userID == "" {
		return mcp.NewToolResultError("unauthorized: missing user context"), nil
	}

	if action == "create" {
		if prompt == "" {
			return mcp.NewToolResultError("prompt is required for creation"), nil
		}

		switch triggerType {
		case "once":
			t, err := time.Parse(time.RFC3339, triggerValue)
			if err != nil {
				// 尝试兼容不带 Z 的格式
				t, err = time.Parse("2006-01-02T15:04:05", triggerValue)
				if err != nil {
					return mcp.NewToolResultError("invalid time format, use ISO8601 (2006-01-02T15:04:05Z)"), nil
				}
			}
			if err := h.jobSvc.CreateOneTimeJob(ctx, userID, targetAgentID, prompt, t); err != nil {
				return mcp.NewToolResultError("failed: " + err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Created One-time Job at %s", t.Format(time.RFC3339))), nil

		case "cron":
			// 简单的 Cron 校验
			if len(triggerValue) < 5 {
				return mcp.NewToolResultError("invalid cron expression"), nil
			}
			if err := h.jobSvc.CreateCronJob(ctx, userID, targetAgentID, prompt, triggerValue); err != nil {
				return mcp.NewToolResultError("failed: " + err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Created Cron Job: %s", triggerValue)), nil

		case "event":
			if triggerValue == "" {
				return mcp.NewToolResultError("event key is required"), nil
			}
			if err := h.jobSvc.CreateEventJob(ctx, userID, targetAgentID, prompt, triggerValue); err != nil {
				return mcp.NewToolResultError("failed: " + err.Error()), nil
			}
			return mcp.NewToolResultText(fmt.Sprintf("Created Event Job: %s", triggerValue)), nil

		default:
			return mcp.NewToolResultError("unknown trigger_type"), nil
		}
	}

	return mcp.NewToolResultError("unknown action"), nil
}

func (h *JobManagementHandler) handleListAgents(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var userID string
	if v := ctx.Value("tenant_user_id"); v != nil {
		if value, ok := v.(string); ok {
			userID = value
		}
	}
	if userID == "" {
		return mcp.NewToolResultError("unauthorized"), nil
	}

	// 调用 Repo 查询 (limit 20)
	agents, err := h.agentRepo.ListAgents(ctx, userID, 20, 0)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}

	// 简化输出
	var summaries []string
	for _, ag := range agents {
		summaries = append(summaries, fmt.Sprintf("ID: %s | Name: %s", ag.AgentId, ag.Name))
	}

	jsonBytes, _ := json.Marshal(summaries)
	return mcp.NewToolResultText(string(jsonBytes)), nil
}

func (h *JobManagementHandler) handleListEvents(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	events := []string{
		"user_login - 用户登录时触发",
		"new_friend_apply - 收到好友申请时触发 (Todo)",
		"group_mention - 群里被@时触发 (Todo)",
	}
	return mcp.NewToolResultText(fmt.Sprintf("Supported Events:\n%v", events)), nil
}
