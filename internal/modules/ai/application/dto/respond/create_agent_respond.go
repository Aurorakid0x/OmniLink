package respond

type CreateAgentRespond struct {
	AgentID   string `json:"agent_id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}
