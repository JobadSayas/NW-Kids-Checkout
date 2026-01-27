package planningcenter

import (
	"time"
)

type eventsResponse struct {
	Links    eventsResponseLinks  `json:"links"`
	Data     []eventsResponseData `json:"data"`
	Included []interface{}        `json:"included"`
	Meta     eventsResponseMeta   `json:"meta"`
}

type eventsResponseLinks struct {
	Self string `json:"self"`
	Next string `json:"next"`
}

type eventsResponseData struct {
	Type          string                      `json:"type"`
	ID            string                      `json:"id"`
	Attributes    eventsResponseAttributes    `json:"attributes"`
	Relationships eventsResponseRelationships `json:"relationships"`
	Links         eventsResponseDataLinks     `json:"links"`
}

type eventsResponseAttributes struct {
	ArchivedAt                *time.Time `json:"archived_at"`
	CreatedAt                 time.Time  `json:"created_at"`
	EnableServicesIntegration bool       `json:"enable_services_integration"`
	Frequency                 string     `json:"frequency"`
	IntegrationKey            *string    `json:"integration_key"`
	LocationTimesEnabled      bool       `json:"location_times_enabled"`
	Name                      string     `json:"name"`
	PreSelectEnabled          bool       `json:"pre_select_enabled"`
	UpdatedAt                 time.Time  `json:"updated_at"`
}

type eventsResponseRelationships struct {
	Campuses eventsResponseCampuses `json:"campuses"`
}

type eventsResponseCampuses struct {
	Data []eventsResponseCampusData `json:"data"`
}

type eventsResponseCampusData struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type eventsResponseDataLinks struct {
	Self string `json:"self"`
	HTML string `json:"html"`
}

type eventsResponseMeta struct {
	TotalCount int                    `json:"total_count"`
	Count      int                    `json:"count"`
	Next       eventsResponseMetaNext `json:"next"`
	CanOrderBy []string               `json:"can_order_by"`
	CanQueryBy []string               `json:"can_query_by"`
	CanInclude []string               `json:"can_include"`
	CanFilter  []string               `json:"can_filter"`
	Parent     eventsResponseParent   `json:"parent"`
}

type eventsResponseMetaNext struct {
	Offset int `json:"offset"`
}

type eventsResponseParent struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}
