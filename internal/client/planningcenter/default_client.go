package planningcenter

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"
)

type defaultClient struct {
	httpClient *http.Client
	baseURL    string
	clientID   string
	secret     string
}

func (client *defaultClient) GetCheckoutsForLocation(ctx context.Context, locationID string, checkedOutOnOrAfter time.Time, limit int) ([]Checkout, error) {
	if checkedOutOnOrAfter.IsZero() && limit == 0 {
		return nil, errors.New("checked_out_on_or_after and limit cannot both be empty")
	}

	data := make([]Checkout, 0)
	iterations := 0

	getURL, err := url.JoinPath(client.baseURL, "check-ins", "v2", "locations", locationID, "check_ins")

	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Add("filter", "checked_out")
	q.Add("order", "-checked_out_at")

	if limit > 0 {
		q.Add("per_page", strconv.Itoa(min(limit, 25)))
	}

	getURL += "?" + q.Encode()

	done := false

	for {
		iterations++
		if iterations >= 10 {
			break
		}
		err := func() error {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
			if err != nil {
				return err
			}

			req.SetBasicAuth(client.clientID, client.secret)
			req.Header.Set("Accept", "application/vnd.api+json")

			resp, err := client.httpClient.Do(req)
			if err != nil {
				return err
			}

			defer resp.Body.Close()

			by, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}

			if resp.StatusCode == http.StatusNotFound {
				return nil
			}

			decoded := checkinResponse{}
			err = json.Unmarshal(by, &decoded)
			if err != nil {
				return err
			}

			for _, item := range decoded.Data {
				if item.Attributes.CheckedOutAt.Before(checkedOutOnOrAfter) {
					done = true
					return nil
				}
				data = append(data, Checkout{
					ID:           item.ID,
					FirstName:    item.Attributes.FirstName,
					LastName:     item.Attributes.LastName,
					CheckedOutAt: item.Attributes.CheckedOutAt,
					SecurityCode: item.Attributes.SecurityCode,
				})
				if limit > 0 && len(data) >= limit {
					done = true
					return nil
				}
			}

			getURL = decoded.Links.Next

			return nil
		}()
		if err != nil {
			return nil, err
		}
		if done {
			break
		}
	}

	return data, nil
}

func (client *defaultClient) GetLocation(ctx context.Context, locationID string, includeAssociatedLocations bool) ([]Location, error) {
	getURL, err := url.JoinPath(client.baseURL, "check-ins", "v2", "locations", locationID)
	if err != nil {
		return nil, err
	}

	if includeAssociatedLocations {
		q := url.Values{}
		q.Add("include", "locations")
		getURL += "?" + q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, getURL, nil)
	if err != nil {
		return nil, err
	}

	req.SetBasicAuth(client.clientID, client.secret)
	req.Header.Set("Accept", "application/vnd.api+json")

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	by, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= http.StatusInternalServerError {
		return nil, &ServerError{
			statusCode: resp.StatusCode,
			errMsg:     string(by),
		}
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return nil, &ClientError{
			statusCode: resp.StatusCode,
			errMsg:     string(by),
		}
	}

	decoded := locationResponse{}
	err = json.Unmarshal(by, &decoded)
	if err != nil {
		return nil, err
	}

	uniqueLocations := map[string]Location{}
	var parentID *string
	if decoded.Data.Relationships.Parent.Data.ID != "" {
		parentID = &decoded.Data.Relationships.Parent.Data.ID
	}
	uniqueLocations[decoded.Data.ID] = Location{
		ID:       decoded.Data.ID,
		Name:     decoded.Data.Attributes.Name,
		ParentID: parentID,
	}

	for _, location := range decoded.Included {
		var incParentID *string
		if location.Relationships.Parent.Data.ID != "" {
			incParentID = &location.Relationships.Parent.Data.ID
		}

		uniqueLocations[location.ID] = Location{
			ID:       location.ID,
			Name:     location.Attributes.Name,
			ParentID: incParentID,
		}
	}

	locations := make([]Location, 0, len(uniqueLocations))
	for _, loc := range uniqueLocations {
		locations = append(locations, loc)
	}

	sort.Slice(locations, func(i, j int) bool {
		if locations[i].ParentID == nil && locations[j].ParentID == nil {
			return locations[i].Name < locations[j].Name
		}

		if locations[i].ParentID != nil && locations[j].ParentID != nil {
			if *locations[i].ParentID != *locations[j].ParentID {
				return *locations[i].ParentID < *locations[j].ParentID
			}
			return locations[i].Name < locations[j].Name
		}
		return locations[i].ParentID == nil
	})

	return locations, nil
}
