package location

import "context"

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
