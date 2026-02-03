package service

import (
	"context"
	"strings"
	"time"

	"OmniLink/internal/modules/ai/application/dto/request"
	"OmniLink/internal/modules/ai/domain/job"
	"OmniLink/internal/modules/ai/domain/repository"
	"OmniLink/pkg/zlog"

	"go.uber.org/zap"
)

type AIJobService interface {
	TriggerByEvent(ctx context.Context, eventKey string, tenantUserID string, vars map[string]string) error
	CreateInstanceFromDef(ctx context.Context, def *job.AIJobDef) error
	ExecuteInstance(ctx context.Context, inst *job.AIJobInst) error
	CreateOneTimeJob(ctx context.Context, userID string, agentID string, prompt string, triggerAt time.Time) error
	CreateCronJob(ctx context.Context, userID string, agentID string, prompt string, cronExpr string) error
	CreateEventJob(ctx context.Context, userID string, agentID string, prompt string, eventKey string) error
	DeactivateJobDef(ctx context.Context, userID string, defID int64) error
}

type aiJobServiceImpl struct {
	jobRepo      repository.AIJobRepository
	assistantSvc AssistantService
}

func NewAIJobService(repo repository.AIJobRepository, as AssistantService) AIJobService {
	return &aiJobServiceImpl{
		jobRepo:      repo,
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
		inst := &job.AIJobInst{
			JobDefID:     def.ID,
			TenantUserID: targetUser,
			AgentID:      def.AgentID,
			Prompt:       finalPrompt,
			Status:       job.JobStatusPending,
			TriggerAt:    time.Now(),
		}
		_ = s.jobRepo.CreateInst(ctx, inst)
	}
	return nil
}

func (s *aiJobServiceImpl) CreateInstanceFromDef(ctx context.Context, def *job.AIJobDef) error {
	inst := &job.AIJobInst{
		JobDefID:     def.ID,
		TenantUserID: def.TenantUserID,
		AgentID:      def.AgentID,
		Prompt:       def.Prompt,
		Status:       job.JobStatusPending,
		TriggerAt:    time.Now(),
	}
	return s.jobRepo.CreateInst(ctx, inst)
}

func (s *aiJobServiceImpl) CreateOneTimeJob(ctx context.Context, userID string, agentID string, prompt string, triggerAt time.Time) error {
	inst := &job.AIJobInst{
		JobDefID:     0,
		TenantUserID: userID,
		AgentID:      agentID,
		Prompt:       prompt,
		Status:       job.JobStatusPending,
		TriggerAt:    triggerAt,
	}
	return s.jobRepo.CreateInst(ctx, inst)
}

func (s *aiJobServiceImpl) CreateCronJob(ctx context.Context, userID string, agentID string, prompt string, cronExpr string) error {
	def := &job.AIJobDef{
		TenantUserID: userID,
		AgentID:      agentID,
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

func (s *aiJobServiceImpl) CreateEventJob(ctx context.Context, userID string, agentID string, prompt string, eventKey string) error {
	def := &job.AIJobDef{
		TenantUserID: userID,
		AgentID:      agentID,
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
		Question: inst.Prompt,
		AgentID:  inst.AgentID,
	}
	_, err := s.assistantSvc.ChatInternal(ctx, req, inst.TenantUserID)
	return err
}
