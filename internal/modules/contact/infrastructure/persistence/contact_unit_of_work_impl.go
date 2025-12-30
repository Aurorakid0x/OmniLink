package persistence

import (
	contactRepository "OmniLink/internal/modules/contact/domain/repository"

	"gorm.io/gorm"
)

type contactUnitOfWorkImpl struct {
	db *gorm.DB
}

func NewContactUnitOfWork(db *gorm.DB) contactRepository.ContactUnitOfWork {
	return &contactUnitOfWorkImpl{db: db}
}

func (u *contactUnitOfWorkImpl) Transaction(fn func(applyRepo contactRepository.ContactApplyRepository, contactRepo contactRepository.UserContactRepository) error) error {
	return u.db.Transaction(func(tx *gorm.DB) error {
		applyRepo := NewContactApplyRepository(tx)
		contactRepo := NewUserContactRepository(tx)
		return fn(applyRepo, contactRepo)
	})
}
