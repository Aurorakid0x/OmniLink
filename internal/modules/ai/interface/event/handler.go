package event

import (
	aiService "OmniLink/internal/modules/ai/application/service"
	userService "OmniLink/internal/modules/user/application/service"
	"OmniLink/pkg/zlog"
	"context"
	"time"

	"go.uber.org/zap"
)

type AIEventHandler struct {
	jobService  aiService.AIJobService
	userService userService.UserInfoService
}

func NewAIEventHandler(jobSvc aiService.AIJobService, userSvc userService.UserInfoService) *AIEventHandler {
	return &AIEventHandler{
		jobService:  jobSvc,
		userService: userSvc,
	}
}

func (h *AIEventHandler) OnUserLogin(ctx context.Context, userID string) {
	zlog.Info("ai event handler: user login", zap.String("user_id", userID))
	vars := map[string]string{
		"login_time": time.Now().Format("2006-01-02 15:04:05"),
	}
	if h.userService != nil {
		info, err := h.userService.GetUserInfoInternal(ctx, userID)
		if err != nil {
			zlog.Error("load user info failed", zap.Error(err), zap.String("user_id", userID))
		} else if info != nil && info.LastOfflineAt != "" {
			vars["last_offline_time"] = info.LastOfflineAt
		}
	}
	go func() {
		if err := h.jobService.TriggerByEvent(context.Background(), "user_login", userID, vars); err != nil {
			zlog.Error("trigger by event failed", zap.Error(err))
		}
	}()
}
