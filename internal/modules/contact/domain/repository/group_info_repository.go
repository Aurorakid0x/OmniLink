package repository

import "OmniLink/internal/modules/contact/domain/entity"

type GroupInfoRepository interface {
	CreateGroupInfo(group *entity.GroupInfo) error
	UpdateGroupInfo(group *entity.GroupInfo) error
	GetGroupInfoByUUID(uuid string) (*entity.GroupInfo, error)
	ListByOwnerID(ownerID string) ([]entity.GroupInfo, error)
	ListJoinedGroups(userID string) ([]entity.GroupInfo, error)
	// SearchGroupsByName 根据群名模糊搜索群组
	SearchGroupsByName(keyword string, limit int) ([]entity.GroupInfo, error)
	// FindGroupByExactName 根据精确群名查找群组
	FindGroupByExactName(name string) (*entity.GroupInfo, error)
}
