package request

type PassContactApplyRequest struct {
	ApplyId string `json:"apply_id"`
	OwnerId string `json:"owner_id"`
}
