package planningcenter

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_defaultClient_GetCheckoutsForLocation_Real(t *testing.T) {
	t.Skip()
	require.NotEmpty(t, os.Getenv("PLANNING_CENTER_API_BASE_URL"), "PLANNING_CENTER_API_BASE_URL must be set to run this test")
	require.NotEmpty(t, os.Getenv("PLANNING_CENTER_API_CLIENT_ID"), "PLANNING_CENTER_API_CLIENT_ID must be set to run this test")
	require.NotEmpty(t, os.Getenv("PLANNING_CENTER_API_SECRET"), "PLANNING_CENTER_API_SECRET must be set to run this test")

	type fields struct {
		baseURL  string
		clientID string
		secret   string
	}
	type args struct {
		ctx                 context.Context
		locationID          string
		checkedOutOnOrAfter time.Time
		limit               int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []Checkout
		wantErr bool
	}{
		{
			name: "get checkins",
			args: args{
				ctx: t.Context(),
				//locationID: "295939",
				locationID:          "723452",
				checkedOutOnOrAfter: time.Date(2025, time.December, 28, 0, 0, 0, 0, time.UTC),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &defaultClient{
				httpClient: http.DefaultClient,
				baseURL:    os.Getenv("PLANNING_CENTER_API_BASE_URL"),
				clientID:   os.Getenv("PLANNING_CENTER_API_CLIENT_ID"),
				secret:     os.Getenv("PLANNING_CENTER_API_SECRET"),
			}
			got, err := client.GetCheckoutsForLocation(tt.args.ctx, tt.args.locationID, tt.args.checkedOutOnOrAfter, tt.args.limit)
			require.NoError(t, err)
			fmt.Printf("%+v\n", got)
		})
	}
}

func Test_defaultClient_GetCheckoutsForLocation_Fake(t *testing.T) {

}

func Test_defaultClient_GetLocation_Real(t *testing.T) {
	t.Skip()
	require.NotEmpty(t, os.Getenv("PLANNING_CENTER_API_BASE_URL"), "PLANNING_CENTER_API_BASE_URL must be set to run this test")
	require.NotEmpty(t, os.Getenv("PLANNING_CENTER_API_CLIENT_ID"), "PLANNING_CENTER_API_CLIENT_ID must be set to run this test")
	require.NotEmpty(t, os.Getenv("PLANNING_CENTER_API_SECRET"), "PLANNING_CENTER_API_SECRET must be set to run this test")

	type fields struct {
		baseURL  string
		clientID string
		secret   string
	}
	type args struct {
		ctx                        context.Context
		locationID                 string
		includeAssociatedLocations bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []Checkout
		wantErr bool
	}{
		{
			name: "get checkins",
			args: args{
				ctx:        t.Context(),
				locationID: "295939",
				//locationID: "723452",
				includeAssociatedLocations: true,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &defaultClient{
				httpClient: http.DefaultClient,
				baseURL:    os.Getenv("PLANNING_CENTER_API_BASE_URL"),
				clientID:   os.Getenv("PLANNING_CENTER_API_CLIENT_ID"),
				secret:     os.Getenv("PLANNING_CENTER_API_SECRET"),
			}
			got, err := client.GetLocation(tt.args.ctx, tt.args.locationID, tt.args.includeAssociatedLocations)
			require.NoError(t, err)
			fmt.Printf("%+v\n", got)
		})
	}
}

func Test_defaultClient_GetLocationsForEvent(t *testing.T) {
	require.NotEmpty(t, os.Getenv("PLANNING_CENTER_API_BASE_URL"), "PLANNING_CENTER_API_BASE_URL must be set to run this test")
	require.NotEmpty(t, os.Getenv("PLANNING_CENTER_API_CLIENT_ID"), "PLANNING_CENTER_API_CLIENT_ID must be set to run this test")
	require.NotEmpty(t, os.Getenv("PLANNING_CENTER_API_SECRET"), "PLANNING_CENTER_API_SECRET must be set to run this test")

	type fields struct {
		baseURL  string
		clientID string
		secret   string
	}
	type args struct {
		ctx                        context.Context
		eventID                    string
		includeAssociatedLocations bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []Checkout
		wantErr bool
	}{
		{
			name: "get checkins",
			args: args{
				ctx:                        t.Context(),
				eventID:                    "152112",
				includeAssociatedLocations: true,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &defaultClient{
				httpClient: http.DefaultClient,
				baseURL:    os.Getenv("PLANNING_CENTER_API_BASE_URL"),
				clientID:   os.Getenv("PLANNING_CENTER_API_CLIENT_ID"),
				secret:     os.Getenv("PLANNING_CENTER_API_SECRET"),
			}
			got, err := client.GetLocationsForEvent(tt.args.ctx, tt.args.eventID)
			require.NoError(t, err)
			fmt.Printf("%+v\n", got)
		})
	}
}

func Test_getFilteredResults(t *testing.T) {
	t1 := time.Now().Add(-10 * time.Minute)
	t2 := t1.Add(1 * time.Second)
	t3 := t2.Add(1 * time.Second)
	t4 := t3.Add(1 * time.Second)
	t5 := t4.Add(1 * time.Second)

	type args struct {
		decoded             checkinResponse
		checkedOutOnOrAfter time.Time
		limit               int
	}
	tests := []struct {
		name     string
		args     args
		wantData []Checkout
		wantDone bool
	}{
		{
			name: "empty decoded",
			args: args{
				decoded:             checkinResponse{},
				checkedOutOnOrAfter: time.Now().AddDate(10, 0, 0),
				limit:               1000,
			},
			wantData: nil,
			wantDone: false,
		},
		{
			name: "decoded",
			args: args{
				decoded: checkinResponse{
					Data: []checkinResponseData{
						{
							ID: "12345",
							Attributes: checkinAttributes{
								FirstName:    "John",
								LastName:     "Doe",
								CheckedOutAt: t5,
								SecurityCode: "ABC123",
							},
						},
						{
							ID: "12346",
							Attributes: checkinAttributes{
								FirstName:    "Mary",
								LastName:     "Sue",
								CheckedOutAt: t5,
								SecurityCode: "ABC124",
							},
						},
						{
							ID: "12347",
							Attributes: checkinAttributes{
								FirstName:    "Jerry",
								LastName:     "Jones",
								CheckedOutAt: t4,
								SecurityCode: "ABC125",
							},
						},
						{
							ID: "12348",
							Attributes: checkinAttributes{
								FirstName:    "Polly",
								LastName:     "Roger",
								CheckedOutAt: t3,
								SecurityCode: "ABC126",
							},
						},
						{
							ID: "12349",
							Attributes: checkinAttributes{
								FirstName:    "Oliver",
								LastName:     "Zamboni",
								CheckedOutAt: t3,
								SecurityCode: "ABC127",
							},
						},
						{
							ID: "12350",
							Attributes: checkinAttributes{
								FirstName:    "Steven",
								LastName:     "Hernandez",
								CheckedOutAt: t2,
								SecurityCode: "ABC128",
							},
						},
						{
							ID: "12351",
							Attributes: checkinAttributes{
								FirstName:    "Samuel",
								LastName:     "Johnson",
								CheckedOutAt: t1,
								SecurityCode: "ABC129",
							},
						},
						{
							ID: "12352",
							Attributes: checkinAttributes{
								FirstName:    "Billy",
								LastName:     "Bob",
								CheckedOutAt: t1.Add(-1 * time.Second),
								SecurityCode: "ABC130",
							},
						},
					},
				},
				checkedOutOnOrAfter: t2,
				limit:               1000,
			},
			wantData: []Checkout{
				{
					ID:           "12345",
					FirstName:    "John",
					LastName:     "Doe",
					CheckedOutAt: t5,
					SecurityCode: "ABC123",
				},
				{
					ID:           "12346",
					FirstName:    "Mary",
					LastName:     "Sue",
					CheckedOutAt: t5,
					SecurityCode: "ABC124",
				},
				{
					ID:           "12347",
					FirstName:    "Jerry",
					LastName:     "Jones",
					CheckedOutAt: t4,
					SecurityCode: "ABC125",
				},
				{
					ID:           "12348",
					FirstName:    "Polly",
					LastName:     "Roger",
					CheckedOutAt: t3,
					SecurityCode: "ABC126",
				},
				{
					ID:           "12349",
					FirstName:    "Oliver",
					LastName:     "Zamboni",
					CheckedOutAt: t3,
					SecurityCode: "ABC127",
				},
				{
					ID:           "12350",
					FirstName:    "Steven",
					LastName:     "Hernandez",
					CheckedOutAt: t2,
					SecurityCode: "ABC128",
				},
				{
					ID:           "12351",
					FirstName:    "Samuel",
					LastName:     "Johnson",
					CheckedOutAt: t1,
					SecurityCode: "ABC129",
				},
			},
			wantDone: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotData, gotDone := getFilteredResults(tt.args.decoded, tt.args.checkedOutOnOrAfter, tt.args.limit)
			assert.Equal(t, tt.wantData, gotData)
			assert.Equal(t, tt.wantDone, gotDone)
		})
	}

}
