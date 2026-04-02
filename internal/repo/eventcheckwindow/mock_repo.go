package eventcheckwindow

import "context"

type MockRepo struct {
	GetCheckWindowByIDFunc          func(ctx context.Context, id int64) (EventCheckWindow, error)
	GetCheckWindowByIDFuncCallCount int

	GetCheckWindowsForEventFunc          func(ctx context.Context, eventID int64) ([]EventCheckWindow, error)
	GetCheckWindowsForEventFuncCallCount int

	ListCheckWindowsFunc          func(ctx context.Context, filter Filter) ([]EventCheckWindow, error)
	ListCheckWindowsFuncCallCount int

	CreateCheckWindowFunc          func(ctx context.Context, window EventCheckWindow) (EventCheckWindow, error)
	CreateCheckWindowFuncCallCount int

	UpdateCheckWindowFunc          func(ctx context.Context, window EventCheckWindow) error
	UpdateCheckWindowFuncCallCount int

	DeleteCheckWindowFunc          func(ctx context.Context, id int64) error
	DeleteCheckWindowFuncCallCount int
}

func (repo *MockRepo) GetCheckWindowByID(ctx context.Context, id int64) (EventCheckWindow, error) {
	repo.GetCheckWindowByIDFuncCallCount++
	if repo.GetCheckWindowByIDFunc != nil {
		return repo.GetCheckWindowByIDFunc(ctx, id)
	}

	panic("MockRepo.GetCheckWindowByID not implemented")
}

func (repo *MockRepo) GetCheckWindowsForEvent(ctx context.Context, eventID int64) ([]EventCheckWindow, error) {
	repo.GetCheckWindowsForEventFuncCallCount++
	if repo.GetCheckWindowsForEventFunc != nil {
		return repo.GetCheckWindowsForEventFunc(ctx, eventID)
	}

	panic("MockRepo.GetCheckWindowsForEvent not implemented")
}

func (repo *MockRepo) ListCheckWindows(ctx context.Context, filter Filter) ([]EventCheckWindow, error) {
	repo.ListCheckWindowsFuncCallCount++
	if repo.ListCheckWindowsFunc != nil {
		return repo.ListCheckWindowsFunc(ctx, filter)
	}

	panic("MockRepo.ListCheckWindows not implemented")
}

func (repo *MockRepo) CreateCheckWindow(ctx context.Context, window EventCheckWindow) (EventCheckWindow, error) {
	repo.CreateCheckWindowFuncCallCount++
	if repo.CreateCheckWindowFunc != nil {
		return repo.CreateCheckWindowFunc(ctx, window)
	}

	panic("MockRepo.CreateCheckWindow not implemented")
}

func (repo *MockRepo) UpdateCheckWindow(ctx context.Context, window EventCheckWindow) error {
	repo.UpdateCheckWindowFuncCallCount++
	if repo.UpdateCheckWindowFunc != nil {
		return repo.UpdateCheckWindowFunc(ctx, window)
	}

	panic("MockRepo.UpdateCheckWindow not implemented")
}

func (repo *MockRepo) DeleteCheckWindow(ctx context.Context, id int64) error {
	repo.DeleteCheckWindowFuncCallCount++
	if repo.DeleteCheckWindowFunc != nil {
		return repo.DeleteCheckWindowFunc(ctx, id)
	}

	panic("MockRepo.DeleteCheckWindow not implemented")
}
