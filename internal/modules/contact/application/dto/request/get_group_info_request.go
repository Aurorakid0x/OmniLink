package request

type GetGroupInfoRequest struct {
	OwnerId string `json:"owner_id"`
	GroupId string `json:"group_id" binding:"required"`
}