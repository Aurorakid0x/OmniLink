package respond

type NewContactApplyItem struct {
	Uuid        string `json:"uuid"`
	UserId      string `json:"user_id"`
	Username    string `json:"username"`
	Nickname    string `json:"nickname"`
	Avatar      string `json:"avatar"`
	Message     string `json:"message"`
	Status      int8   `json:"status"`
	LastApplyAt string `json:"last_apply_at"`
}
