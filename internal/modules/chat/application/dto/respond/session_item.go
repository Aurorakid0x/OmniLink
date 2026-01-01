package respond

type SessionItem struct {
	SessionId   string `json:"session_id"`
	SendId      string `json:"send_id,omitempty"`
	ReceiveId   string `json:"receive_id,omitempty"`
	ReceiveName string `json:"receive_name,omitempty"`
	Avatar      string `json:"avatar,omitempty"`
	PeerId      string `json:"peer_id,omitempty"`
	PeerType    string `json:"peer_type,omitempty"`
	PeerName    string `json:"peer_name,omitempty"`
	PeerAvatar  string `json:"peer_avatar,omitempty"`
	UpdatedAt   string `json:"updated_at,omitempty"`
	LastMsg     string `json:"last_msg,omitempty"`
	UnreadCount int    `json:"unread_count,omitempty"`
}
