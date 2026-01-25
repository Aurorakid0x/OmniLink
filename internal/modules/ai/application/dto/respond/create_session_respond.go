package respond

type CreateSessionRespond struct {
	SessionID string `json:"session_id"`
	Title     string `json:"title"`
	AgentID   string `json:"agent_id"`
	CreatedAt string `json:"created_at"`
}
