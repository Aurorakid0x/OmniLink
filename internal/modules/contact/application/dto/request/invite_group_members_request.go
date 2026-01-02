package request

type InviteGroupMembersRequest struct {
	OwnerId   string   `json:"owner_id"`
	GroupId   string   `json:"group_id" binding:"required"`
	MemberIds []string `json:"member_ids" binding:"required,min=1"`
}