package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"OmniLink/internal/modules/ai/application/service"
	"OmniLink/internal/modules/ai/domain/job"
	"OmniLink/internal/modules/ai/domain/repository"
	"OmniLink/pkg/zlog"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

type JobManagementHandler struct {
	jobSvc      service.AIJobService
	agentRepo   repository.AgentRepository
	sessionRepo repository.AssistantSessionRepository
}

func NewJobManagementHandler(svc service.AIJobService, agentRepo repository.AgentRepository, sessionRepo repository.AssistantSessionRepository) *JobManagementHandler {
	return &JobManagementHandler{
		jobSvc:      svc,
		agentRepo:   agentRepo,
		sessionRepo: sessionRepo,
	}
}

func (h *JobManagementHandler) RegisterTools(s *server.MCPServer) {
	// 1. Tool: manage_ai_job (核心创建/删除工具)
	s.AddTool(mcp.NewTool("manage_ai_job",
		mcp.WithDescription("创建或管理 AI 自动化任务。支持定时(cron)、一次性(once)、事件驱动(event)三种模式。用于生成发送给Agent执行的指令。"),
		mcp.WithString("action", mcp.Required(), mcp.Description("操作类型: create | delete")),
		mcp.WithString("trigger_type", mcp.Description("触发类型: once | cron | event")),
		mcp.WithString("trigger_value", mcp.Description("触发值: once传ISO时间(2006-01-02T15:04:05Z), cron传5段表达式(0 8 * * *), event传事件key(仅支持: user_login, new_friend_apply, group_mention，使用list_supported_events查看完整列表)")),
		mcp.WithString("task_description", mcp.Description("【可选】任务的完整描述，仅用于帮助你理解上下文，不会存储到数据库。例如: '每天8点提醒用户喝水'、'用户登录时查询待办事项'。这个参数可以包含时间、触发条件等完整信息，用于你理解用户意图。")),
		mcp.WithString("prompt", mcp.Description(`任务执行时发送给 AI 的指令（纯执行动作，不含触发条件）

【格式要求】
- 只描述要做什么（What to do）
- 不要包含何时做（When）、为什么做（Why）、触发条件

【正确示例】
✅ "提醒用户喝水，发送友好的消息"
✅ "查询用户今日待办事项列表并告知"
✅ "调用 weather 工具获取天气信息并推送给用户"
✅ "检查用户是否有未读消息，如果有则提醒"

【错误示例】
❌ "每天8点提醒用户喝水"（包含时间，应去掉"每天8点"）
❌ "用户登录时查询待办事项"（包含触发条件，应去掉"用户登录时"）
❌ "因为用户设置了提醒，所以发送消息"（包含原因，应只写"发送消息"）
❌ "在9:00提醒开会"（包含时间，应改为"提醒用户开会"）

【提示】触发条件已由 trigger_type 和 trigger_value 定义，prompt 只需关注执行逻辑`)),
		mcp.WithString("agent_id", mcp.Description("执行任务的AgentID (可选，默认使用当前Agent)")),
		mcp.WithNumber("job_def_id", mcp.Description("删除任务时传入的任务定义ID")),
	), h.handleManageJob)

	// 2. Tool: list_my_agents (辅助工具，查询用户有哪些Agent)
	s.AddTool(mcp.NewTool("list_my_agents",
		mcp.WithDescription("列出当前用户拥有的所有 Agent，用于获取 agent_id"),
	), h.handleListAgents)

	// 3. Tool: list_supported_events (辅助工具，查询支持哪些事件)
	s.AddTool(mcp.NewTool("list_supported_events",
		mcp.WithDescription("列出系统支持的所有触发事件 Key"),
	), h.handleListEvents)

	s.AddTool(mcp.NewTool("get_current_time",
		mcp.WithDescription("获取当前系统时间"),
	), h.handleGetCurrentTime)
}

func (h *JobManagementHandler) handleManageJob(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args, ok := request.Params.Arguments.(map[string]interface{})
	if !ok {
		zlog.Warn("manage_ai_job invalid arguments")
		return mcp.NewToolResultError("invalid arguments format"), nil
	}
	action, _ := args["action"].(string)
	triggerType, _ := args["trigger_type"].(string)
	triggerValue, _ := args["trigger_value"].(string)
	prompt, _ := args["prompt"].(string)
	targetAgentID, _ := args["agent_id"].(string)
	var defID int64
	if rawID, ok := args["job_def_id"]; ok {
		if v, ok := rawID.(float64); ok {
			defID = int64(v)
		}
	}

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
		zlog.Warn("manage_ai_job missing user context",
			zap.String("action", action),
			zap.String("trigger_type", triggerType))
		return mcp.NewToolResultError("unauthorized: missing user context"), nil
	}

	zlog.Info("manage_ai_job request",
		zap.String("user_id", userID),
		zap.String("action", action),
		zap.String("trigger_type", triggerType),
		zap.String("trigger_value", triggerValue),
		zap.String("agent_id", targetAgentID),
		zap.Int("prompt_len", len(prompt)))

	if action == "create" {
		if prompt == "" {
			return mcp.NewToolResultError("prompt is required for creation"), nil
		}
		if triggerType == "" {
			return mcp.NewToolResultError("trigger_type is required for creation"), nil
		}
		if targetAgentID == "" {
			return mcp.NewToolResultError("agent_id is required"), nil
		}
		// 校验 agent 归属，防止越权调用
		ag, err := h.agentRepo.GetAgentByID(ctx, targetAgentID, userID)
		if err != nil {
			zlog.Warn("manage_ai_job agent lookup failed", zap.Error(err), zap.String("user_id", userID), zap.String("agent_id", targetAgentID))
			return mcp.NewToolResultError("agent lookup failed: " + err.Error()), nil
		}
		if ag == nil {
			zlog.Warn("manage_ai_job agent not found", zap.String("user_id", userID), zap.String("agent_id", targetAgentID))
			return mcp.NewToolResultError("agent_id not found or unauthorized"), nil
		}
		if h.sessionRepo == nil {
			zlog.Warn("manage_ai_job session repo is nil", zap.String("user_id", userID))
			return mcp.NewToolResultError("session repo is nil"), nil
		}
		session, err := h.sessionRepo.GetSystemGlobalSession(ctx, userID)
		if err != nil {
			zlog.Warn("manage_ai_job get system session failed", zap.Error(err), zap.String("user_id", userID))
			return mcp.NewToolResultError("failed to get system session: " + err.Error()), nil
		}
		if session == nil {
			zlog.Warn("manage_ai_job system session not found", zap.String("user_id", userID))
			return mcp.NewToolResultError("system session not found"), nil
		}
		if strings.TrimSpace(session.AgentId) == "" {
			zlog.Warn("manage_ai_job system session missing agent_id", zap.String("user_id", userID), zap.String("session_id", session.SessionId))
			return mcp.NewToolResultError("system session missing agent_id"), nil
		}
		if targetAgentID != "" && strings.TrimSpace(session.AgentId) != targetAgentID {
			zlog.Warn("manage_ai_job agent mismatch", zap.String("user_id", userID), zap.String("session_agent_id", session.AgentId), zap.String("agent_id", targetAgentID))
			return mcp.NewToolResultError("agent_id does not match system session agent_id"), nil
		}
		if targetAgentID == "" {
			targetAgentID = strings.TrimSpace(session.AgentId)
		}
		sessionID := strings.TrimSpace(session.SessionId)
		if sessionID == "" {
			zlog.Warn("manage_ai_job system session id empty", zap.String("user_id", userID))
			return mcp.NewToolResultError("system session_id is empty"), nil
		}

		zlog.Info("manage_ai_job create resolved",
			zap.String("user_id", userID),
			zap.String("agent_id", targetAgentID),
			zap.String("session_id", sessionID),
			zap.String("trigger_type", triggerType),
			zap.String("trigger_value", triggerValue))
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
			if err := h.jobSvc.CreateOneTimeJob(ctx, userID, targetAgentID, sessionID, prompt, t); err != nil {
				zlog.Warn("manage_ai_job create once failed", zap.Error(err), zap.String("user_id", userID), zap.String("agent_id", targetAgentID), zap.String("session_id", sessionID))
				return mcp.NewToolResultError("failed: " + err.Error()), nil
			}
			zlog.Info("manage_ai_job create once success",
				zap.String("user_id", userID),
				zap.String("agent_id", targetAgentID),
				zap.String("session_id", sessionID),
				zap.String("trigger_at", t.Format(time.RFC3339)))
			return mcp.NewToolResultText(fmt.Sprintf("Created One-time Job at %s", t.Format(time.RFC3339))), nil

		case "cron":
			// 使用标准5段cron表达式校验
			if _, err := cron.ParseStandard(triggerValue); err != nil {
				zlog.Warn("manage_ai_job invalid cron", zap.Error(err), zap.String("user_id", userID), zap.String("cron", triggerValue))
				return mcp.NewToolResultError("invalid cron expression (5 fields required)"), nil
			}
			if err := h.jobSvc.CreateCronJob(ctx, userID, targetAgentID, sessionID, prompt, triggerValue); err != nil {
				zlog.Warn("manage_ai_job create cron failed", zap.Error(err), zap.String("user_id", userID), zap.String("agent_id", targetAgentID), zap.String("session_id", sessionID), zap.String("cron", triggerValue))
				return mcp.NewToolResultError("failed: " + err.Error()), nil
			}
			zlog.Info("manage_ai_job create cron success",
				zap.String("user_id", userID),
				zap.String("agent_id", targetAgentID),
				zap.String("session_id", sessionID),
				zap.String("cron", triggerValue))
			return mcp.NewToolResultText(fmt.Sprintf("Created Cron Job: %s", triggerValue)), nil

		case "event":
			if triggerValue == "" {
				zlog.Warn("manage_ai_job missing event key", zap.String("user_id", userID))
				return mcp.NewToolResultError("event key is required"), nil
			}
			if !job.IsValidEventKey(triggerValue) {
				supported := make([]string, 0, len(job.AllSupportedEvents()))
				for k := range job.AllSupportedEvents() {
					supported = append(supported, k)
				}
				zlog.Warn("manage_ai_job invalid event key",
					zap.String("user_id", userID),
					zap.String("event_key", triggerValue),
					zap.Strings("supported", supported))
				return mcp.NewToolResultError(
					fmt.Sprintf("不支持的 event_key: %s，仅支持: %s", triggerValue, strings.Join(supported, ", "))), nil
			}
			if err := h.jobSvc.CreateEventJob(ctx, userID, targetAgentID, sessionID, prompt, triggerValue); err != nil {
				zlog.Warn("manage_ai_job create event failed", zap.Error(err), zap.String("user_id", userID), zap.String("agent_id", targetAgentID), zap.String("session_id", sessionID), zap.String("event_key", triggerValue))
				return mcp.NewToolResultError("failed: " + err.Error()), nil
			}
			zlog.Info("manage_ai_job create event success",
				zap.String("user_id", userID),
				zap.String("agent_id", targetAgentID),
				zap.String("session_id", sessionID),
				zap.String("event_key", triggerValue))
			return mcp.NewToolResultText(fmt.Sprintf("Created Event Job: %s", triggerValue)), nil

		default:
			return mcp.NewToolResultError("unknown trigger_type"), nil
		}
	}

	if action == "delete" {
		if defID <= 0 {
			return mcp.NewToolResultError("job_def_id is required for delete"), nil
		}
		// 软删除任务定义：保留历史实例
		if err := h.jobSvc.DeactivateJobDef(ctx, userID, defID); err != nil {
			zlog.Warn("manage_ai_job delete failed", zap.Error(err), zap.String("user_id", userID), zap.Int64("job_def_id", defID))
			return mcp.NewToolResultError("failed: " + err.Error()), nil
		}
		zlog.Info("manage_ai_job delete success", zap.String("user_id", userID), zap.Int64("job_def_id", defID))
		return mcp.NewToolResultText(fmt.Sprintf("Deactivated Job Def: %d", defID)), nil
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
	events := job.AllSupportedEvents()
	var lines []string
	for key, desc := range events {
		lines = append(lines, fmt.Sprintf("%s - %s", key, desc))
	}
	return mcp.NewToolResultText(fmt.Sprintf("Supported Events:\n%s", strings.Join(lines, "\n"))), nil
}

func (h *JobManagementHandler) handleGetCurrentTime(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	now := time.Now()
	return mcp.NewToolResultJSON(map[string]interface{}{
		"rfc3339":  now.Format(time.RFC3339),
		"unix":     now.Unix(),
		"timezone": now.Location().String(),
	})
}
