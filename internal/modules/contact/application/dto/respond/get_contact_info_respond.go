package respond

type GetContactInfoRespond struct {
	ContactId        string `json:"contact_id"`
	ContactName      string `json:"contact_name"`
	ContactAvatar    string `json:"contact_avatar"`
	ContactSignature string `json:"contact_signature"`
	Gender           int8   `json:"gender"`
	Birthday         string `json:"birthday"`
}