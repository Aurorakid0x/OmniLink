package repository

type ContactUnitOfWork interface {
	Transaction(fn func(applyRepo ContactApplyRepository, contactRepo UserContactRepository, groupRepo GroupInfoRepository) error) error
}
