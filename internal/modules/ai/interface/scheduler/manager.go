package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"OmniLink/internal/modules/ai/application/service"
	"OmniLink/internal/modules/ai/domain/job"
	"OmniLink/internal/modules/ai/domain/repository"
	"OmniLink/pkg/zlog"

	"github.com/robfig/cron/v3"
)

type SchedulerManager struct {
	cron       *cron.Cron
	jobRepo    repository.AIJobRepository
	jobService service.AIJobService
	stopChan   chan struct{}
	mu         sync.Mutex
	scheduled  map[int64]cron.EntryID
}

func NewSchedulerManager(repo repository.AIJobRepository, svc service.AIJobService) *SchedulerManager {
	return &SchedulerManager{
		// 使用标准5段Cron表达式（不含秒）
		cron:       cron.New(),
		jobRepo:    repo,
		jobService: svc,
		stopChan:   make(chan struct{}),
		scheduled:  make(map[int64]cron.EntryID),
	}
}

func (m *SchedulerManager) Start() {
	// 启动时先加载已存在的Cron规则
	m.refreshCronJobs()
	m.cron.Start()
	go m.runPoller()
	zlog.Info("AI Job Interface (Scheduler) started")
}

func (m *SchedulerManager) Stop() {
	m.cron.Stop()
	close(m.stopChan)
}

func (m *SchedulerManager) refreshCronJobs() {
	ctx := context.Background()
	defs, err := m.jobRepo.GetActiveCronDefs(ctx)
	if err != nil {
		return
	}

	active := make(map[int64]*job.AIJobDef, len(defs))
	for _, def := range defs {
		if def == nil {
			continue
		}
		active[def.ID] = def
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for defID, entryID := range m.scheduled {
		if _, ok := active[defID]; !ok {
			// 已被停用的规则，从调度器中移除
			m.cron.Remove(entryID)
			delete(m.scheduled, defID)
		}
	}

	for defID, def := range active {
		if _, ok := m.scheduled[defID]; ok {
			continue
		}
		d := def
		entryID, err := m.cron.AddFunc(d.CronExpr, func() {
			// Cron触发只负责创建实例，具体执行由轮询器完成
			bgCtx := context.Background()
			_ = m.jobService.CreateInstanceFromDef(bgCtx, d)
		})
		if err != nil {
			zlog.Error("cron schedule failed: " + err.Error())
			continue
		}
		m.scheduled[defID] = entryID
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
	// 高频刷新Cron规则，保证新建/停用尽快生效
	m.refreshCronJobs()
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
			summary, err := m.jobService.ExecuteInstance(ctx, i)
			if err != nil {
				if i.RetryCount >= 3 {
					_ = m.jobRepo.UpdateInstStatus(ctx, i.ID, job.JobStatusFailed, err.Error())
				} else {
					_ = m.jobRepo.IncrInstRetry(ctx, i.ID)
					// 退避重试：避免高频失败导致写库和执行压力
					nextAt := time.Now().Add(time.Duration(i.RetryCount+1) * 30 * time.Second)
					_ = m.jobRepo.UpdateInstForRetry(ctx, i.ID, nextAt, "Retry pending")
				}
			} else {
				if summary == "" {
					summary = "Success"
				}
				_ = m.jobRepo.UpdateInstStatus(ctx, i.ID, job.JobStatusCompleted, summary)
			}
		}(inst)
	}
}
