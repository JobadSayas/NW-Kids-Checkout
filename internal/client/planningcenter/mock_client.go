package planningcenter

import (
	"context"
	"time"
)

type MockClient struct {
	GetCheckoutsForEventFunc          func(ctx context.Context, eventID string, checkedOutOnOrAfter time.Time, limit int) ([]Checkout, error)
	GetCheckoutsForEventFuncCallCount uint

	GetLocationFunc          func(ctx context.Context, locationID string, includeAssociatedLocations bool) ([]Location, error)
	GetLocationFuncCallCount uint

	GetLocationsForEventFunc          func(ctx context.Context, eventID string) ([]Location, error)
	GetLocationsForEventFuncCallCount uint

	GetEventByIDFunc          func(ctx context.Context, eventID string) (Event, error)
	GetEventByIDFuncCallCount uint

	GetEventsFunc          func(ctx context.Context) ([]Event, string, error)
	GetEventsFuncCallCount uint

	GetEventsFromNextURLFunc          func(ctx context.Context, nextURL string) ([]Event, string, error)
	GetEventsFromNextURLFuncCallCount uint
}

func (client *MockClient) GetLocationsForEvent(ctx context.Context, eventID string) ([]Location, error) {
	client.GetLocationsForEventFuncCallCount++
	if client.GetLocationsForEventFunc != nil {
		return client.GetLocationsForEventFunc(ctx, eventID)
	}

	panic("MockClient.GetLocationsForEvent not implemented")
}

func (client *MockClient) GetEventByID(ctx context.Context, eventID string) (Event, error) {
	client.GetEventByIDFuncCallCount++
	if client.GetEventByIDFunc != nil {
		return client.GetEventByIDFunc(ctx, eventID)
	}

	panic("MockClient.GetEventByID not implemented")
}

func (client *MockClient) GetCheckoutsForEvent(ctx context.Context, eventID string, checkedOutOnOrAfter time.Time, limit int) ([]Checkout, error) {
	client.GetCheckoutsForEventFuncCallCount++
	if client.GetCheckoutsForEventFunc != nil {
		return client.GetCheckoutsForEventFunc(ctx, eventID, checkedOutOnOrAfter, limit)
	}

	panic("MockClient.GetCheckoutsForEvent not implemented")
}

func (client *MockClient) GetLocation(ctx context.Context, locationID string, includeAssociatedLocations bool) ([]Location, error) {
	client.GetLocationFuncCallCount++
	if client.GetLocationFunc != nil {
		return client.GetLocationFunc(ctx, locationID, includeAssociatedLocations)
	}

	panic("MockClient.GetLocation not implemented")
}

func (client *MockClient) GetEvents(ctx context.Context) ([]Event, string, error) {
	client.GetEventsFuncCallCount++
	if client.GetEventsFunc != nil {
		return client.GetEventsFunc(ctx)
	}

	panic("MockClient.GetEvents not implemented")
}

func (client *MockClient) GetEventsFromNextURL(ctx context.Context, nextURL string) ([]Event, string, error) {
	client.GetEventsFromNextURLFuncCallCount++
	if client.GetEventsFromNextURLFunc != nil {
		return client.GetEventsFromNextURLFunc(ctx, nextURL)
	}

	panic("MockClient.GetEventsFromNextURL not implemented")
}
