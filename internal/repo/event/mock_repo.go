package event

import "context"

type MockRepo struct {
	GetEventByIDFunc          func(ctx context.Context, id int64) (Event, error)
	GetEventByIDFuncCallCount int

	GetEventByPlanningCenterIDFunc          func(ctx context.Context, planningCenterID string) (Event, error)
	GetEventByPlanningCenterIDFuncCallCount int

	CreateEventFunc          func(ctx context.Context, event Event) (Event, error)
	CreateEventFuncCallCount int

	UpdateEventNameFunc          func(ctx context.Context, id int64, name string) error
	UpdateEventNameFuncCallCount int
}

func (repo *MockRepo) GetEventByID(ctx context.Context, id int64) (Event, error) {
	repo.GetEventByIDFuncCallCount++
	if repo.GetEventByIDFunc != nil {
		return repo.GetEventByIDFunc(ctx, id)
	}

	panic("MockRepo.GetEventByID not implemented")
}

func (repo *MockRepo) CreateEvent(ctx context.Context, event Event) (Event, error) {
	repo.CreateEventFuncCallCount++
	if repo.CreateEventFunc != nil {
		return repo.CreateEventFunc(ctx, event)
	}

	panic("MockRepo.CreateEvent not implemented")
}

func (repo *MockRepo) UpdateEventName(ctx context.Context, id int64, name string) error {
	repo.UpdateEventNameFuncCallCount++
	if repo.UpdateEventNameFunc != nil {
		return repo.UpdateEventNameFunc(ctx, id, name)
	}

	panic("MockRepo.UpdateEventName not implemented")
}

func (repo *MockRepo) GetEventByPlanningCenterID(ctx context.Context, planningCenterID string) (Event, error) {
	repo.GetEventByPlanningCenterIDFuncCallCount++
	if repo.GetEventByPlanningCenterIDFunc != nil {
		return repo.GetEventByPlanningCenterIDFunc(ctx, planningCenterID)
	}

	panic("MockRepo.GetEventByPlanningCenterID not implemented")
}
