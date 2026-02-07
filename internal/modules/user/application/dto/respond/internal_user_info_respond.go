package respond

type InternalUserInfoRespond struct {
	Id            int64  `json:"id"`
	Uuid          string `json:"uuid"`
	Username      string `json:"username"`
	Nickname      string `json:"nickname"`
	Avatar        string `json:"avatar"`
	Gender        int8   `json:"gender"`
	Signature     string `json:"signature"`
	Birthday      string `json:"birthday"`
	CreatedAt     string `json:"created_at"`
	LastOnlineAt  string `json:"last_online_at"`
	LastOfflineAt string `json:"last_offline_at"`
	IsAdmin       int8   `json:"is_admin"`
	Status        int8   `json:"status"`
}
