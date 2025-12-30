package request

type ApplyContactRequest struct {
	OwnerId     string `json:"owner_id"`
	ContactId   string `json:"contact_id"`
	ContactType int8   `json:"contact_type"`
	Message     string `json:"message"`
}
