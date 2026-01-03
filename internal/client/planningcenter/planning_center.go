package planningcenter

import (
	"context"
	"net/http"
	"os"
	"time"
)

type Checkout struct {
	ID           string
	FirstName    string
	LastName     string
	CheckedOutAt time.Time
	SecurityCode string
}

type Location struct {
	ID       string
	ParentID *string
	Name     string
}

type Client interface {
	GetCheckoutsForLocation(ctx context.Context, locationID string, checkedOutOnOrAfter time.Time, limit int) ([]Checkout, error)
	GetLocation(ctx context.Context, locationID string, includeAssociatedLocations bool) ([]Location, error)
}

func NewClient() Client {
	return &defaultClient{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL:  os.Getenv("PLANNING_CENTER_API_BASE_URL"),
		clientID: os.Getenv("PLANNING_CENTER_API_CLIENT_ID"),
		secret:   os.Getenv("PLANNING_CENTER_API_SECRET"),
	}
}

type checkinResponse struct {
	Links struct {
		Self string `json:"self"`
		Next string `json:"next"`
	} `json:"links"`
	Data []checkinResponseData `json:"data"`
}

type checkinResponseData struct {
	Type          string               `json:"type"`
	ID            string               `json:"id"`
	Attributes    checkinAttributes    `json:"attributes"`
	Relationships checkinRelationships `json:"relationships"`
}

type checkinAttributes struct {
	FirstName                   string    `json:"first_name"`
	LastName                    string    `json:"last_name"`
	MedicalNotes                string    `json:"medical_notes"`
	Number                      int       `json:"number"`
	SecurityCode                string    `json:"security_code"`
	CreatedAt                   time.Time `json:"created_at"`
	UpdatedAt                   time.Time `json:"updated_at"`
	CheckedOutAt                time.Time `json:"checked_out_at"`
	ConfirmedAt                 time.Time `json:"confirmed_at"`
	EmergencyContactName        string    `json:"emergency_contact_name"`
	EmergencyContactPhoneNumber string    `json:"emergency_contact_phone_number"`
	OneTimeGuest                bool      `json:"one_time_guest"`
	Kind                        string    `json:"kind"`
}

type checkinRelationships struct {
	EventPeriod relationshipData `json:"event_period"`
	Person      relationshipData `json:"person"`
}

type relationshipData struct {
	Data relationInfo `json:"data"`
}

type relationInfo struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}
