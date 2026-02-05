package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"OmniLink/internal/modules/ai/application/dto/request"
	"OmniLink/internal/modules/ai/domain/assistant"
	"OmniLink/internal/modules/ai/domain/job"
	"OmniLink/internal/modules/ai/domain/repository"
	"OmniLink/pkg/zlog"

	"go.uber.org/zap"
)

type AIJobService interface {
	TriggerByEvent(ctx context.Context, eventKey string, tenantUserID string, vars map[string]string) error
	CreateInstanceFromDef(ctx context.Context, def *job.AIJobDef) error
	ExecuteInstance(ctx context.Context, inst *job.AIJobInst) error
	CreateOneTimeJob(ctx context.Context, userID string, agentID string, sessionID string, prompt string, triggerAt time.Time) error
	CreateCronJob(ctx context.Context, userID string, agentID string, sessionID string, prompt string, cronExpr string) error
	CreateEventJob(ctx context.Context, userID string, agentID string, sessionID string, prompt string, eventKey string) error
	DeactivateJobDef(ctx context.Context, userID string, defID int64) error
}

type aiJobServiceImpl struct {
	jobRepo      repository.AIJobRepository
	sessionRepo  repository.AssistantSessionRepository
	assistantSvc AssistantService
}

func NewAIJobService(repo repository.AIJobRepository, sessionRepo repository.AssistantSessionRepository, as AssistantService) AIJobService {
	return &aiJobServiceImpl{
		jobRepo:      repo,
		sessionRepo:  sessionRepo,
		assistantSvc: as,
	}
}

func (s *aiJobServiceImpl) TriggerByEvent(ctx context.Context, eventKey string, tenantUserID string, vars map[string]string) error {
	defs, err := s.jobRepo.GetDefsByEventAndUser(ctx, eventKey, tenantUserID)
	if err != nil {
		return err
	}
	globalDefs, err := s.jobRepo.GetDefsByEvent(ctx, eventKey)
	if err == nil {
		defs = append(defs, globalDefs...)
	}
	if len(defs) == 0 {
		return nil
	}
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
	return s.jobRepo.CreateInst(ctx, inst)
}

func (s *aiJobServiceImpl) CreateOneTimeJob(ctx context.Context, userID string, agentID string, sessionID string, prompt string, triggerAt time.Time) error {
	if err := s.validateSession(ctx, userID, agentID, sessionID); err != nil {
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
	return s.jobRepo.CreateInst(ctx, inst)
}

func (s *aiJobServiceImpl) CreateCronJob(ctx context.Context, userID string, agentID string, sessionID string, prompt string, cronExpr string) error {
	if err := s.validateSession(ctx, userID, agentID, sessionID); err != nil {
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
	return s.jobRepo.CreateDef(ctx, def)
}

func (s *aiJobServiceImpl) CreateEventJob(ctx context.Context, userID string, agentID string, sessionID string, prompt string, eventKey string) error {
	if err := s.validateSession(ctx, userID, agentID, sessionID); err != nil {
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
	return s.jobRepo.CreateDef(ctx, def)
}

func (s *aiJobServiceImpl) DeactivateJobDef(ctx context.Context, userID string, defID int64) error {
	if userID == "" || defID <= 0 {
		return nil
	}
	// 仅停用规则，不删除历史实例
	return s.jobRepo.DeactivateDef(ctx, defID, userID)
}

func (s *aiJobServiceImpl) ExecuteInstance(ctx context.Context, inst *job.AIJobInst) error {
	zlog.Info("executing ai job instance", zap.Int64("inst_id", inst.ID))
	req := request.AssistantChatRequest{
		Question:  inst.Prompt,
		AgentID:   inst.AgentID,
		SessionID: inst.SessionID,
	}
	_, err := s.assistantSvc.ChatInternal(ctx, req, inst.TenantUserID)
	return err
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
