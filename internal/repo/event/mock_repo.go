package event

import (
	"context"
	"sync/atomic"
)

type MockRepo struct {
	GetEventByIDFunc          func(ctx context.Context, id int64) (Event, error)
	GetEventByIDFuncCallCount atomic.Int64

	GetEventByPlanningCenterIDFunc          func(ctx context.Context, planningCenterID string) (Event, error)
	GetEventByPlanningCenterIDFuncCallCount atomic.Int64

	ListEventsFunc          func(ctx context.Context, filter EventFilter) ([]Event, error)
	ListEventsFuncCallCount atomic.Int64

	CreateEventFunc          func(ctx context.Context, event Event) (Event, error)
	CreateEventFuncCallCount atomic.Int64

	UpdateEventFunc          func(ctx context.Context, event Event) error
	UpdateEventFuncCallCount atomic.Int64

	DeleteEventFunc          func(ctx context.Context, id int64) error
	DeleteEventFuncCallCount atomic.Int64
}

func (repo *MockRepo) GetEventByID(ctx context.Context, id int64) (Event, error) {
	repo.GetEventByIDFuncCallCount.Add(1)
	if repo.GetEventByIDFunc != nil {
		return repo.GetEventByIDFunc(ctx, id)
	}

	panic("MockRepo.GetEventByID not implemented")
}

func (repo *MockRepo) CreateEvent(ctx context.Context, event Event) (Event, error) {
	repo.CreateEventFuncCallCount.Add(1)
	if repo.CreateEventFunc != nil {
		return repo.CreateEventFunc(ctx, event)
	}

	panic("MockRepo.CreateEvent not implemented")
}

func (repo *MockRepo) UpdateEvent(ctx context.Context, event Event) error {
	repo.UpdateEventFuncCallCount.Add(1)
	if repo.UpdateEventFunc != nil {
		return repo.UpdateEventFunc(ctx, event)
	}

	panic("MockRepo.UpdateEvent not implemented")
}

func (repo *MockRepo) GetEventByPlanningCenterID(ctx context.Context, planningCenterID string) (Event, error) {
	repo.GetEventByPlanningCenterIDFuncCallCount.Add(1)
	if repo.GetEventByPlanningCenterIDFunc != nil {
		return repo.GetEventByPlanningCenterIDFunc(ctx, planningCenterID)
	}

	panic("MockRepo.GetEventByPlanningCenterID not implemented")
}

func (repo *MockRepo) ListEvents(ctx context.Context, filter EventFilter) ([]Event, error) {
	repo.ListEventsFuncCallCount.Add(1)
	if repo.ListEventsFunc != nil {
		return repo.ListEventsFunc(ctx, filter)
	}

	panic("MockRepo.ListEvents not implemented")
}

func (repo *MockRepo) DeleteEvent(ctx context.Context, id int64) error {
	repo.DeleteEventFuncCallCount.Add(1)
	if repo.DeleteEventFunc != nil {
		return repo.DeleteEventFunc(ctx, id)
	}

	panic("MockRepo.DeleteEvent not implemented")
}
