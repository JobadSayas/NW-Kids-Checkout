package event

import "context"

type MockRepo struct {
	GetEventByIDFunc          func(ctx context.Context, id int64) (Event, error)
	GetEventByIDFuncCallCount int
}

func (repo *MockRepo) GetEventByID(ctx context.Context, id int64) (Event, error) {
	repo.GetEventByIDFuncCallCount++
	if repo.GetEventByIDFunc != nil {
		return repo.GetEventByIDFunc(ctx, id)
	}

	panic("MockRepo.GetEventByID not implemented")
}
