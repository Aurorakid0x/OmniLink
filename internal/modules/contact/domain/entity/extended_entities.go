package entity

type ContactWithUserInfo struct {
	UserContact
	Nickname  string `gorm:"column:nickname"`
	Avatar    string `gorm:"column:avatar"`
	Signature string `gorm:"column:signature"`
}