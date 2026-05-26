package http

import (
	"ardoise/apps/backend/internal/core/application"
	"ardoise/apps/backend/internal/core/mocks"
)

func newTestServices(eRepo *mocks.MockExpenseRepo, uRepo *mocks.MockUserRepo, gRepo *mocks.MockGroupRepo, aRepo *mocks.MockAuditRepo) (*application.ExpenseService, *application.UserService, *application.GroupService) {
	tx := &mocks.MockTransactor{}
	es := application.NewExpenseService(eRepo, gRepo, aRepo, tx)
	us := application.NewUserService(uRepo, []byte("test-secret"))
	gs := application.NewGroupService(gRepo, eRepo, aRepo, tx)
	return es, us, gs
}
