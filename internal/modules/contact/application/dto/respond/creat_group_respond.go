package respond

type CreateGroupRespond struct {
	Uuid      string `json:"uuid"`
	GroupId   string `json:"group_id"`
	Name      string `json:"name"`
	Notice    string `json:"notice"`
	OwnerId   string `json:"owner_id"`
	MemberCnt int    `json:"member_cnt"`
	Avatar    string `json:"avatar"`
	Status    int8   `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}
