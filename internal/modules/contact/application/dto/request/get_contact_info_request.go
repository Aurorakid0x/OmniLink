package request

type GetContactInfoRequest struct {
	ContactId string `json:"contact_id"`
	OwnerId   string `json:"-"`
}
