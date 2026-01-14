package checkin

import "context"

type MockRepo struct {
	ListCheckinsFunc          func(ctx context.Context, filter Filter) ([]Checkin, error)
	ListCheckinsFuncCallCount int

	CreateCheckinFunc          func(ctx context.Context, checkin Checkin) (Checkin, error)
	CreateCheckinFuncCallCount int
}

func (repo *MockRepo) ListCheckins(ctx context.Context, filter Filter) ([]Checkin, error) {
	repo.ListCheckinsFuncCallCount++
	if repo.ListCheckinsFunc != nil {
		return repo.ListCheckinsFunc(ctx, filter)
	}
	panic("MockRepo.ListCheckins not implemented")
}

func (repo *MockRepo) CreateCheckin(ctx context.Context, checkin Checkin) (Checkin, error) {
	repo.CreateCheckinFuncCallCount++
	if repo.CreateCheckinFunc != nil {
		return repo.CreateCheckinFunc(ctx, checkin)
	}
	panic("MockRepo.CreateCheckin not implemented")
}
