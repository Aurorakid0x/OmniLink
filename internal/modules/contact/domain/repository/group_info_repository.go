package repository

import "OmniLink/internal/modules/contact/domain/entity"

type GroupInfoRepository interface {
	CreateGroupInfo(group *entity.GroupInfo) error
	UpdateGroupInfo(group *entity.GroupInfo) error
	GetGroupInfoByUUID(uuid string) (*entity.GroupInfo, error)
	ListByOwnerID(ownerID string) ([]entity.GroupInfo, error)
}