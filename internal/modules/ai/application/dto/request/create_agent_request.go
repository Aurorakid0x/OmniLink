package request

type CreateAgentRequest struct {
	Name          string `json:"name" binding:"required"`
	Description   string `json:"description"`
	PersonaPrompt string `json:"persona_prompt"`
	KBType        string `json:"kb_type" binding:"required,oneof=global agent_private"`
	KBName        string `json:"kb_name"` // 仅当 KBType=agent_private 时需要
}
