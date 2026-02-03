package event

import (
	"OmniLink/internal/modules/ai/application/service"
	"OmniLink/pkg/zlog"
	"context"
	"time"

	"go.uber.org/zap"
)

type AIEventHandler struct {
	jobService service.AIJobService
}

func NewAIEventHandler(svc service.AIJobService) *AIEventHandler {
	return &AIEventHandler{jobService: svc}
}

func (h *AIEventHandler) OnUserLogin(ctx context.Context, userID string) {
	zlog.Info("ai event handler: user login", zap.String("user_id", userID))
	vars := map[string]string{
		"login_time": time.Now().Format("2006-01-02 15:04:05"),
	}
	go func() {
		if err := h.jobService.TriggerByEvent(context.Background(), "user_login", userID, vars); err != nil {
			zlog.Error("trigger by event failed", zap.Error(err))
		}
	}()
}
