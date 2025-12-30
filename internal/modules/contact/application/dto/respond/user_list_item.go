package respond

type UserListItem struct {
	UserId   string `json:"user_id"`
	UserName string `json:"user_name"`
	Avatar   string `json:"avatar"`
	Status   int8   `json:"status"`
}
