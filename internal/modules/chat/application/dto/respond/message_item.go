package respond

type MessageItem struct {
	Uuid       string `json:"uuid"`
	SessionId  string `json:"session_id,omitempty"`
	SendId     string `json:"send_id"`
	SendName   string `json:"send_name,omitempty"`
	SendAvatar string `json:"send_avatar,omitempty"`
	ReceiveId  string `json:"receive_id"`
	Type       int8   `json:"type"`
	Content    string `json:"content,omitempty"`
	Url        string `json:"url,omitempty"`
	FileType   string `json:"file_type,omitempty"`
	FileName   string `json:"file_name,omitempty"`
	FileSize   string `json:"file_size,omitempty"`
	CreatedAt  string `json:"created_at"`
}
