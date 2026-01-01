package request

type GetMessageListRequest struct {
	UserOneId string `json:"user_one_id"`
	UserTwoId string `json:"user_two_id"`
	Page      int    `json:"page"`
	PageSize  int    `json:"page_size"`
}
