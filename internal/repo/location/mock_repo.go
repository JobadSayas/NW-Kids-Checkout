package location

import (
	"context"

	"kids-checkin/internal/repo"
)

type MockRepo struct {
	ListLocationsFunc          func(ctx context.Context, filter LocationFilter) ([]Location, error)
	listLocationsFuncCallCount int

	CreateLocationFunc          func(ctx context.Context, location Location) (Location, error)
	CreateLocationFuncCallCount int

	UpdateLocationFunc          func(ctx context.Context, location Location) error
	UpdateLocationFuncCallCount int

	DeleteLocationFunc          func(ctx context.Context, id int64) error
	DeleteLocationFuncCallCount int

	ListLocationGroupsFunc          func(ctx context.Context, filter LocationGroupFilter) ([]LocationGroup, error)
	ListLocationGroupsFuncCallCount int

	CreateLocationGroupFunc          func(ctx context.Context, lg LocationGroup) (LocationGroup, error)
	CreateLocationGroupFuncCallCount int

	UpdateLocationGroupFunc          func(ctx context.Context, lg LocationGroup) error
	UpdateLocationGroupFuncCallCount int

	DeleteLocationGroupFunc          func(ctx context.Context, id int64) error
	DeleteLocationGroupFuncCallCount int
}

func (repo *MockRepo) ListLocations(ctx context.Context, filter LocationFilter) ([]Location, error) {
	repo.listLocationsFuncCallCount++
	if repo.ListLocationsFunc != nil {
		return repo.ListLocationsFunc(ctx, filter)
	}
	panic("MockRepo.ListLocations not implemented")
}

func (repo *MockRepo) CreateLocation(ctx context.Context, location Location) (Location, error) {
	repo.CreateLocationFuncCallCount++
	if repo.CreateLocationFunc != nil {
		return repo.CreateLocationFunc(ctx, location)
	}
	panic("MockRepo.CreateLocation not implemented")
}

func (repo *MockRepo) UpdateLocation(ctx context.Context, location Location) error {
	repo.UpdateLocationFuncCallCount++
	if repo.UpdateLocationFunc != nil {
		return repo.UpdateLocationFunc(ctx, location)
	}
	panic("MockRepo.UpdateLocation not implemented")
}

func (repo *MockRepo) DeleteLocation(ctx context.Context, id int64) error {
	repo.DeleteLocationFuncCallCount++
	if repo.DeleteLocationFunc != nil {
		return repo.DeleteLocationFunc(ctx, id)
	}
	panic("MockRepo.DeleteLocation not implemented")
}

func (repo *MockRepo) ListLocationGroups(ctx context.Context, filter LocationGroupFilter) ([]LocationGroup, error) {
	if repo.ListLocationGroupsFunc != nil {
		return repo.ListLocationGroupsFunc(ctx, filter)
	}

	panic("MockRepo.ListLocationGroups not implemented")
}

func (repo *MockRepo) CreateLocationGroup(ctx context.Context, lg LocationGroup) (LocationGroup, error) {
	repo.CreateLocationGroupFuncCallCount++
	if repo.CreateLocationGroupFunc != nil {
		return repo.CreateLocationGroupFunc(ctx, lg)
	}
	panic("MockRepo.CreateLocationGroup not implemented")
}

func (m *MockRepo) UpdateLocationGroup(ctx context.Context, lg LocationGroup) error {
	m.UpdateLocationGroupFuncCallCount++
	if m.UpdateLocationGroupFunc != nil {
		return m.UpdateLocationGroupFunc(ctx, lg)
	}
	return repo.ErrNotFound
}

func (m *MockRepo) DeleteLocationGroup(ctx context.Context, id int64) error {
	m.DeleteLocationGroupFuncCallCount++
	if m.DeleteLocationGroupFunc != nil {
		return m.DeleteLocationGroupFunc(ctx, id)
	}
	return ErrLocationGroupInUse
}
