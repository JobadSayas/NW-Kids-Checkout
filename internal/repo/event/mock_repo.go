package event

import "context"

type MockRepo struct {
	GetEventByIDFunc          func(ctx context.Context, id int64) (Event, error)
	GetEventByIDFuncCallCount int

	GetEventByPlanningCenterIDFunc          func(ctx context.Context, planningCenterID string) (Event, error)
	GetEventByPlanningCenterIDFuncCallCount int

	ListEventsFunc          func(ctx context.Context, filter EventFilter) ([]Event, error)
	ListEventsFuncCallCount int

	CreateEventFunc          func(ctx context.Context, event Event) (Event, error)
	CreateEventFuncCallCount int

	UpdateEventFunc          func(ctx context.Context, event Event) error
	UpdateEventFuncCallCount int
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

func (repo *MockRepo) UpdateEvent(ctx context.Context, event Event) error {
	repo.UpdateEventFuncCallCount++
	if repo.UpdateEventFunc != nil {
		return repo.UpdateEventFunc(ctx, event)
	}

	panic("MockRepo.UpdateEvent not implemented")
}

func (repo *MockRepo) GetEventByPlanningCenterID(ctx context.Context, planningCenterID string) (Event, error) {
	repo.GetEventByPlanningCenterIDFuncCallCount++
	if repo.GetEventByPlanningCenterIDFunc != nil {
		return repo.GetEventByPlanningCenterIDFunc(ctx, planningCenterID)
	}

	panic("MockRepo.GetEventByPlanningCenterID not implemented")
}

func (repo *MockRepo) ListEvents(ctx context.Context, filter EventFilter) ([]Event, error) {
	repo.ListEventsFuncCallCount++
	if repo.ListEventsFunc != nil {
		return repo.ListEventsFunc(ctx, filter)
	}

	panic("MockRepo.ListEvents not implemented")
}
