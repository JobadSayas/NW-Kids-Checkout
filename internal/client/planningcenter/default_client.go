package planningcenter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"time"
)

type defaultClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

func (client *defaultClient) GetCheckoutsForLocation(ctx context.Context, locationID string, olderThan time.Time) ([]Checkout, error) {
	getURL, err := url.JoinPath(client.baseURL, "check-ins", "v2", "locations", locationID, "check_ins")
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Add("checked_out", "true")
	getURL += "?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	decoded := make([]checkinResponse, 0)
	err = json.NewDecoder(resp.Body).Decode(&decoded)
	if err != nil {
		return nil, err
	}

	data := make([]Checkout, 0, len(decoded))
	for _, item := range decoded {
		data = append(data, Checkout{
			ID:           item.ID,
			FirstName:    item.Attributes.FirstName,
			LastName:     item.Attributes.LastName,
			CheckedOutAt: item.Attributes.CheckedOutAt,
			SecurityCode: item.Attributes.SecurityCode,
		})
	}
	return data, nil
}
