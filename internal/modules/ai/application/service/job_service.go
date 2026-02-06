package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"OmniLink/internal/modules/ai/domain/assistant"
	"OmniLink/internal/modules/ai/domain/job"
	"OmniLink/internal/modules/ai/domain/repository"
	"OmniLink/internal/modules/ai/infrastructure/pipeline"
	"OmniLink/pkg/zlog"

	"go.uber.org/zap"
)

type AIJobService interface {
	TriggerByEvent(ctx context.Context, eventKey string, tenantUserID string, vars map[string]string) error
	CreateInstanceFromDef(ctx context.Context, def *job.AIJobDef) error
	ExecuteInstance(ctx context.Context, inst *job.AIJobInst) (string, error)
	CreateOneTimeJob(ctx context.Context, userID string, agentID string, sessionID string, prompt string, triggerAt time.Time) error
	CreateCronJob(ctx context.Context, userID string, agentID string, sessionID string, prompt string, cronExpr string) error
	CreateEventJob(ctx context.Context, userID string, agentID string, sessionID string, prompt string, eventKey string) error
	DeactivateJobDef(ctx context.Context, userID string, defID int64) error
}

type aiJobServiceImpl struct {
	jobRepo     repository.AIJobRepository
	sessionRepo repository.AssistantSessionRepository
	jobExecPipe *pipeline.JobExecutionPipeline
}

func NewAIJobService(repo repository.AIJobRepository, sessionRepo repository.AssistantSessionRepository, jobExecPipe *pipeline.JobExecutionPipeline) AIJobService {
	return &aiJobServiceImpl{
		jobRepo:     repo,
		sessionRepo: sessionRepo,
		jobExecPipe: jobExecPipe,
	}
}

func (s *aiJobServiceImpl) TriggerByEvent(ctx context.Context, eventKey string, tenantUserID string, vars map[string]string) error {
	defs, err := s.jobRepo.GetDefsByEventAndUser(ctx, eventKey, tenantUserID)
	if err != nil {
		zlog.Warn("ai job trigger by event failed", zap.Error(err), zap.String("event_key", eventKey), zap.String("tenant_user_id", tenantUserID))
		return err
	}
	globalDefs, err := s.jobRepo.GetDefsByEvent(ctx, eventKey)
	if err == nil {
		defs = append(defs, globalDefs...)
	}
	if len(defs) == 0 {
		zlog.Info("ai job trigger by event no defs", zap.String("event_key", eventKey), zap.String("tenant_user_id", tenantUserID))
		return nil
	}
	zlog.Info("ai job trigger by event",
		zap.String("event_key", eventKey),
		zap.String("tenant_user_id", tenantUserID),
		zap.Int("defs", len(defs)))
	for _, def := range defs {
		finalPrompt := def.Prompt
		for k, v := range vars {
			finalPrompt = strings.ReplaceAll(finalPrompt, "{{"+k+"}}", v)
		}
		targetUser := def.TenantUserID
		if targetUser == "" {
			targetUser = tenantUserID
		}
		if strings.TrimSpace(def.SessionID) == "" {
			return errors.New("session_id is required for job execution")
		}
		if err := s.validateSession(ctx, targetUser, def.AgentID, def.SessionID); err != nil {
			zlog.Warn("ai job trigger by event session invalid",
				zap.Error(err),
				zap.String("event_key", eventKey),
				zap.String("tenant_user_id", targetUser),
				zap.String("agent_id", def.AgentID),
				zap.String("session_id", def.SessionID))
			return err
		}
		inst := &job.AIJobInst{
			JobDefID:     def.ID,
			TenantUserID: targetUser,
			AgentID:      def.AgentID,
			SessionID:    def.SessionID,
			Prompt:       finalPrompt,
			Status:       job.JobStatusPending,
			TriggerAt:    time.Now(),
		}
		zlog.Info("ai job instance created from event",
			zap.Int64("job_def_id", def.ID),
			zap.String("tenant_user_id", targetUser),
			zap.String("agent_id", def.AgentID),
			zap.String("session_id", def.SessionID),
			zap.Int("prompt_len", len(finalPrompt)))
		_ = s.jobRepo.CreateInst(ctx, inst)
	}
	return nil
}

func (s *aiJobServiceImpl) CreateInstanceFromDef(ctx context.Context, def *job.AIJobDef) error {
	if def == nil {
		return nil
	}
	if strings.TrimSpace(def.SessionID) == "" {
		return errors.New("session_id is required for job execution")
	}
	if err := s.validateSession(ctx, def.TenantUserID, def.AgentID, def.SessionID); err != nil {
		zlog.Warn("ai job create instance validate failed",
			zap.Error(err),
			zap.Int64("job_def_id", def.ID),
			zap.String("tenant_user_id", def.TenantUserID),
			zap.String("agent_id", def.AgentID),
			zap.String("session_id", def.SessionID))
		return err
	}
	inst := &job.AIJobInst{
		JobDefID:     def.ID,
		TenantUserID: def.TenantUserID,
		AgentID:      def.AgentID,
		SessionID:    def.SessionID,
		Prompt:       def.Prompt,
		Status:       job.JobStatusPending,
		TriggerAt:    time.Now(),
	}
	zlog.Info("ai job instance created from def",
		zap.Int64("job_def_id", def.ID),
		zap.String("tenant_user_id", def.TenantUserID),
		zap.String("agent_id", def.AgentID),
		zap.String("session_id", def.SessionID),
		zap.Int("prompt_len", len(def.Prompt)))
	return s.jobRepo.CreateInst(ctx, inst)
}

func (s *aiJobServiceImpl) CreateOneTimeJob(ctx context.Context, userID string, agentID string, sessionID string, prompt string, triggerAt time.Time) error {
	if err := s.validateSession(ctx, userID, agentID, sessionID); err != nil {
		zlog.Warn("ai job create one time validate failed",
			zap.Error(err),
			zap.String("tenant_user_id", userID),
			zap.String("agent_id", agentID),
			zap.String("session_id", sessionID))
		return err
	}
	inst := &job.AIJobInst{
		JobDefID:     0,
		TenantUserID: userID,
		AgentID:      agentID,
		SessionID:    sessionID,
		Prompt:       prompt,
		Status:       job.JobStatusPending,
		TriggerAt:    triggerAt,
	}
	zlog.Info("ai job create one time",
		zap.String("tenant_user_id", userID),
		zap.String("agent_id", agentID),
		zap.String("session_id", sessionID),
		zap.String("trigger_at", triggerAt.Format(time.RFC3339)),
		zap.Int("prompt_len", len(prompt)))
	return s.jobRepo.CreateInst(ctx, inst)
}

func (s *aiJobServiceImpl) CreateCronJob(ctx context.Context, userID string, agentID string, sessionID string, prompt string, cronExpr string) error {
	if err := s.validateSession(ctx, userID, agentID, sessionID); err != nil {
		zlog.Warn("ai job create cron validate failed",
			zap.Error(err),
			zap.String("tenant_user_id", userID),
			zap.String("agent_id", agentID),
			zap.String("session_id", sessionID))
		return err
	}
	def := &job.AIJobDef{
		TenantUserID: userID,
		AgentID:      agentID,
		SessionID:    sessionID,
		TriggerType:  job.TriggerTypeCron,
		CronExpr:     cronExpr,
		Prompt:       prompt,
		IsActive:     true,
		Title:        "Scheduled Task: " + cronExpr,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	zlog.Info("ai job create cron",
		zap.String("tenant_user_id", userID),
		zap.String("agent_id", agentID),
		zap.String("session_id", sessionID),
		zap.String("cron", cronExpr),
		zap.Int("prompt_len", len(prompt)))
	return s.jobRepo.CreateDef(ctx, def)
}

func (s *aiJobServiceImpl) CreateEventJob(ctx context.Context, userID string, agentID string, sessionID string, prompt string, eventKey string) error {
	if err := s.validateSession(ctx, userID, agentID, sessionID); err != nil {
		zlog.Warn("ai job create event validate failed",
			zap.Error(err),
			zap.String("tenant_user_id", userID),
			zap.String("agent_id", agentID),
			zap.String("session_id", sessionID))
		return err
	}
	def := &job.AIJobDef{
		TenantUserID: userID,
		AgentID:      agentID,
		SessionID:    sessionID,
		TriggerType:  job.TriggerTypeEvent,
		EventKey:     eventKey,
		Prompt:       prompt,
		IsActive:     true,
		Title:        "Event Task: " + eventKey,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	zlog.Info("ai job create event",
		zap.String("tenant_user_id", userID),
		zap.String("agent_id", agentID),
		zap.String("session_id", sessionID),
		zap.String("event_key", eventKey),
		zap.Int("prompt_len", len(prompt)))
	return s.jobRepo.CreateDef(ctx, def)
}

func (s *aiJobServiceImpl) DeactivateJobDef(ctx context.Context, userID string, defID int64) error {
	if userID == "" || defID <= 0 {
		return nil
	}
	// 仅停用规则，不删除历史实例
	return s.jobRepo.DeactivateDef(ctx, defID, userID)
}

func (s *aiJobServiceImpl) ExecuteInstance(ctx context.Context, inst *job.AIJobInst) (string, error) {
	zlog.Info("executing ai job instance", zap.Int64("inst_id", inst.ID))
	if inst == nil {
		return "", errors.New("job instance is nil")
	}
	zlog.Info("ai job execute start",
		zap.Int64("inst_id", inst.ID),
		zap.Int64("job_def_id", inst.JobDefID),
		zap.String("tenant_user_id", inst.TenantUserID),
		zap.String("agent_id", inst.AgentID),
		zap.String("session_id", inst.SessionID),
		zap.Int("prompt_len", len(inst.Prompt)))

	req := &pipeline.JobExecutionRequest{
		TenantUserID: inst.TenantUserID,
		AgentID:      inst.AgentID,
		SessionID:    inst.SessionID,
		Prompt:       inst.Prompt,
		TopK:         5,
	}
	result, err := s.jobExecPipe.Execute(ctx, req)
	if err != nil {
		zlog.Warn("ai job execute failed",
			zap.Error(err),
			zap.Int64("inst_id", inst.ID),
			zap.String("tenant_user_id", inst.TenantUserID),
			zap.String("session_id", inst.SessionID))
		return "", err
	}
	if result != nil && result.Err != nil {
		zlog.Warn("ai job execute result error",
			zap.Error(result.Err),
			zap.Int64("inst_id", inst.ID),
			zap.String("tenant_user_id", inst.TenantUserID),
			zap.String("session_id", inst.SessionID))
		return "", result.Err
	}

	summary := "Success"
	if result != nil && result.ResultSummary != "" {
		summary = result.ResultSummary
	}

	zlog.Info("ai job execute done",
		zap.Int64("inst_id", inst.ID),
		zap.String("tenant_user_id", inst.TenantUserID),
		zap.String("session_id", inst.SessionID),
		zap.Int("answer_len", len(result.Answer)),
		zap.String("summary", summary))
	return summary, nil
}

func (s *aiJobServiceImpl) validateSession(ctx context.Context, userID string, agentID string, sessionID string) error {
	userID = strings.TrimSpace(userID)
	agentID = strings.TrimSpace(agentID)
	sessionID = strings.TrimSpace(sessionID)
	if userID == "" || agentID == "" || sessionID == "" {
		return errors.New("user_id, agent_id and session_id are required")
	}
	if s.sessionRepo == nil {
		return errors.New("session repo is nil")
	}
	session, err := s.sessionRepo.GetSessionByID(ctx, sessionID, userID)
	if err != nil {
		return err
	}
	if session == nil {
		return errors.New("session not found or unauthorized")
	}
	if strings.TrimSpace(session.AgentId) != agentID {
		return errors.New("session agent_id mismatch")
	}
	if strings.TrimSpace(session.SessionType) != assistant.SessionTypeSystemGlobal {
		return errors.New("session_type must be system_global")
	}
	return nil
}
