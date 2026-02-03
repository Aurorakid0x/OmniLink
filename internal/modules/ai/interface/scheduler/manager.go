package scheduler

import (
	"OmniLink/internal/modules/ai/application/service"
	"OmniLink/internal/modules/ai/domain/job"
	"OmniLink/internal/modules/ai/domain/repository"
	"OmniLink/pkg/zlog"
	"context"
	"fmt"
	"time"

	"github.com/robfig/cron/v3"
)

type SchedulerManager struct {
	cron       *cron.Cron
	jobRepo    repository.AIJobRepository
	jobService service.AIJobService
	stopChan   chan struct{}
}

func NewSchedulerManager(repo repository.AIJobRepository, svc service.AIJobService) *SchedulerManager {
	return &SchedulerManager{
		cron:       cron.New(cron.WithSeconds()),
		jobRepo:    repo,
		jobService: svc,
		stopChan:   make(chan struct{}),
	}
}

func (m *SchedulerManager) Start() {
	m.loadAndScheduleCronJobs()
	m.cron.Start()
	go m.runPoller()
	zlog.Info("AI Job Interface (Scheduler) started")
}

func (m *SchedulerManager) Stop() {
	m.cron.Stop()
	close(m.stopChan)
}

func (m *SchedulerManager) loadAndScheduleCronJobs() {
	ctx := context.Background()
	defs, err := m.jobRepo.GetActiveCronDefs(ctx)
	if err != nil {
		return
	}
	for _, def := range defs {
		d := def
		_, _ = m.cron.AddFunc(d.CronExpr, func() {
			bgCtx := context.Background()
			_ = m.jobService.CreateInstanceFromDef(bgCtx, d)
		})
	}
}

func (m *SchedulerManager) runPoller() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.pollAndExecute()
		case <-m.stopChan:
			return
		}
	}
}

func (m *SchedulerManager) pollAndExecute() {
	ctx := context.Background()
	insts, err := m.jobRepo.GetPendingInsts(ctx, 10)
	if err != nil || len(insts) == 0 {
		return
	}
	for _, inst := range insts {
		go func(i *job.AIJobInst) {
			defer func() {
				if r := recover(); r != nil {
					_ = m.jobRepo.UpdateInstStatus(ctx, i.ID, job.JobStatusFailed, fmt.Sprintf("Panic: %v", r))
				}
			}()
			if err := m.jobRepo.UpdateInstStatus(ctx, i.ID, job.JobStatusRunning, ""); err != nil {
				return
			}
			err := m.jobService.ExecuteInstance(ctx, i)
			if err != nil {
				if i.RetryCount >= 3 {
					_ = m.jobRepo.UpdateInstStatus(ctx, i.ID, job.JobStatusFailed, err.Error())
				} else {
					_ = m.jobRepo.IncrInstRetry(ctx, i.ID)
					_ = m.jobRepo.UpdateInstStatus(ctx, i.ID, job.JobStatusPending, "Retry pending")
				}
			} else {
				_ = m.jobRepo.UpdateInstStatus(ctx, i.ID, job.JobStatusCompleted, "Success")
			}
		}(inst)
	}
}
