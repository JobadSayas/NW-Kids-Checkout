package planningcenter

import (
	"context"
	"time"
)

type MockClient struct {
	GetCheckoutsForLocationFunc          func(ctx context.Context, locationID string, olderThan time.Time) ([]Checkout, error)
	GetCheckoutsForLocationFuncCallCount uint

	GetLocationFunc          func(ctx context.Context, locationID string, includeAssociatedLocations bool) ([]Location, error)
	GetLocationFuncCallCount uint

	GetLocationsForEventFunc          func(ctx context.Context, eventID string) ([]Location, error)
	GetLocationsForEventFuncCallCount uint
}

func (client *MockClient) GetLocationsForEvent(ctx context.Context, eventID string) ([]Location, error) {
	client.GetLocationsForEventFuncCallCount++
	if client.GetLocationsForEventFunc != nil {
		return client.GetLocationsForEventFunc(ctx, eventID)
	}

	panic("MockClient.GetLocationsForEvent not implemented")
}

func (client *MockClient) GetCheckoutsForLocation(ctx context.Context, locationID string, checkedOutOnOrAfter time.Time, limit int) ([]Checkout, error) {
	client.GetCheckoutsForLocationFuncCallCount++
	if client.GetCheckoutsForLocationFunc != nil {
		return client.GetCheckoutsForLocationFunc(ctx, locationID, checkedOutOnOrAfter)
	}

	panic("MockClient.GetCheckoutsForLocation not implemented")
}

func (client *MockClient) GetLocation(ctx context.Context, locationID string, includeAssociatedLocations bool) ([]Location, error) {
	client.GetLocationFuncCallCount++
	if client.GetLocationFunc != nil {
		return client.GetLocationFunc(ctx, locationID, includeAssociatedLocations)
	}

	panic("MockClient.GetLocation not implemented")
}
