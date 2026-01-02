package respond

type GroupMemberRespond struct {
	UserId   string `json:"user_id"`
	Username string `json:"username"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Gender   int8   `json:"gender"`
	Role     int8   `json:"role"` // 0: member, 1: owner
}