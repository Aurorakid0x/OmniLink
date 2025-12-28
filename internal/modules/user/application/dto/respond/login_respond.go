package respond

type LoginRespond struct {
	Uuid      string `json:"uuid"`
	Username  string `json:"username"`
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar"`
	Gender    int8   `json:"gender"`
	Birthday  string `json:"birthday"`
	Signature string `json:"signature"`
	CreatedAt string `json:"created_at"`
	IsAdmin   int8   `json:"is_admin"`
	Status    int8   `json:"status"`
	Token     string `json:"token"`
}
