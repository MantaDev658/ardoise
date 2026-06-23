package handlers

import (
	"ardoise/apps/backend/internal/core/application"
	"ardoise/apps/backend/internal/core/mocks"
)

func newTestServices(eRepo *mocks.MockExpenseRepo, uRepo *mocks.MockUserRepo, gRepo *mocks.MockGroupRepo) (*application.ExpenseService, *application.UserService, *application.GroupService) {
	tx := &mocks.MockTransactor{}
	es := application.NewExpenseService(eRepo, gRepo, tx)
	us := application.NewUserService(uRepo, []byte("test-secret"))
	gs := application.NewGroupService(gRepo, eRepo, &mocks.MockInvitationRepo{}, uRepo, tx)
	return es, us, gs
}
