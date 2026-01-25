package request

type CreateSessionRequest struct {
	AgentID string `json:"agent_id" binding:"required"`
	Title   string `json:"title"` // 可选
}
