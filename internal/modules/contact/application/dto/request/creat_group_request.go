package request

type CreateGroupRequest struct {
	OwnerId   string   `json:"owner_id"`
	Name      string   `json:"name"`
	Notice    string   `json:"notice"`
	MemberIds []string `json:"member_ids"`
}
