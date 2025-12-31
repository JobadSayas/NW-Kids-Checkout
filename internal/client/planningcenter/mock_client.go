package planningcenter

import (
	"context"
	"time"
)

type MockClient struct {
	GetCheckoutsForLocationFunc          func(ctx context.Context, locationID string, olderThan time.Time) ([]Checkout, error)
	GetCheckoutsForLocationFuncCallCount uint
}

func (client *MockClient) GetCheckoutsForLocation(ctx context.Context, locationID string, olderThan time.Time) ([]Checkout, error) {
	client.GetCheckoutsForLocationFuncCallCount++
	if client.GetCheckoutsForLocationFunc != nil {
		return client.GetCheckoutsForLocationFunc(ctx, locationID, olderThan)
	}

	panic("MockClient.GetCheckoutsForLocation not implemented")
}
