package planningcenter

type eventByIDResponse struct {
	Data eventByIDResponseData `json:"data"`
}

type eventByIDResponseData struct {
	ID         string                     `json:"id"`
	Type       string                     `json:"type"`
	Attributes eventByIDResponseAttribute `json:"attributes"`
}

type eventByIDResponseAttribute struct {
	Name string `json:"name"`
}
