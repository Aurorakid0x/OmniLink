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
}
