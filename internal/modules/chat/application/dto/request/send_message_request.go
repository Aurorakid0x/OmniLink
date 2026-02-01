package request

type SendMessageRequest struct {
	SessionId string `json:"session_id"`
	ReceiveId string `json:"receive_id"`
	Type      int8   `json:"type"`
	Content   string `json:"content"`
	Url       string `json:"url"`

	FileType string `json:"file_type"`
	FileName string `json:"file_name"`
	FileSize string `json:"file_size"`

	MentionedUserIds []string `json:"mentioned_user_ids"` // 被提及的用户ID列表
	MentionAll       bool     `json:"mention_all"`        // 是否提及所有人
}
