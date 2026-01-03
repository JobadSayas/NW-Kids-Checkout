package planningcenter

import "time"

type locationResponse struct {
	Data     LocationTopLevelData `json:"data"`
	Included []LocationIncluded   `json:"included"`
	Meta     LocationMeta         `json:"meta"`
}
type LocationIncludedAttributes struct {
	AgeMaxInMonths        any       `json:"age_max_in_months"`
	AgeMinInMonths        any       `json:"age_min_in_months"`
	AgeOn                 any       `json:"age_on"`
	AgeRangeBy            any       `json:"age_range_by"`
	AttendeesPerVolunteer any       `json:"attendees_per_volunteer"`
	ChildOrAdult          any       `json:"child_or_adult"`
	CreatedAt             time.Time `json:"created_at"`
	EffectiveDate         any       `json:"effective_date"`
	Gender                any       `json:"gender"`
	GradeMax              any       `json:"grade_max"`
	GradeMin              any       `json:"grade_min"`
	Kind                  string    `json:"kind"`
	MaxOccupancy          any       `json:"max_occupancy"`
	Milestone             any       `json:"milestone"`
	MinVolunteers         any       `json:"min_volunteers"`
	Name                  string    `json:"name"`
	Opened                bool      `json:"opened"`
	Position              int       `json:"position"`
	Questions             []any     `json:"questions"`
	UpdatedAt             time.Time `json:"updated_at"`
}
type LocationParent struct {
	Data struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	} `json:"data"`
}
type Links struct {
	Related string `json:"related"`
}
type Data struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}
type Locations struct {
	Links Links  `json:"links"`
	Data  []Data `json:"data"`
}
type LocationRelationships struct {
	Parent    LocationParent `json:"parent"`
	Locations Locations      `json:"locations"`
}
type LocationLinks struct {
	CheckIns             string `json:"check_ins"`
	Event                string `json:"event"`
	IntegrationLinks     string `json:"integration_links"`
	LocationEventPeriods string `json:"location_event_periods"`
	LocationEventTimes   string `json:"location_event_times"`
	LocationLabels       string `json:"location_labels"`
	Locations            string `json:"locations"`
	Options              string `json:"options"`
	Parent               any    `json:"parent"`
	Self                 string `json:"self"`
}
type LocationTopLevelData struct {
	Type          string                     `json:"type"`
	ID            string                     `json:"id"`
	Attributes    LocationIncludedAttributes `json:"attributes"`
	Relationships LocationRelationships      `json:"relationships"`
	Links         LocationLinks              `json:"links"`
}
type LocationIncludedRelationships struct {
	Parent Parent `json:"parent"`
}
type LocationIncludedLinks struct {
	Self string `json:"self"`
}
type LocationIncluded struct {
	Type          string                        `json:"type"`
	ID            string                        `json:"id"`
	Attributes    LocationIncludedAttributes    `json:"attributes"`
	Relationships LocationIncludedRelationships `json:"relationships"`
	Links         LocationIncludedLinks         `json:"links"`
}
type Parent struct {
	Data struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	} `json:"data"`
}
type LocationMeta struct {
	CanInclude []string `json:"can_include"`
	Parent     Parent   `json:"parent"`
}
