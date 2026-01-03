package planningcenter

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Test_defaultClient_GetCheckoutsForLocation(t *testing.T) {
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

func Test_defaultClient_GetLocation(t *testing.T) {
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
