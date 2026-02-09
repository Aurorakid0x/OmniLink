package request

type PredictRequest struct {
	Input   string                 `json:"input"`
	Context map[string]interface{} `json:"context"`
}

type PolishRequest struct {
	Text    string                 `json:"text"`
	Context map[string]interface{} `json:"context"`
}

type DigestRequest struct {
	GroupId      string    `json:"group_id"`
	MessageCount int       `json:"message_count"`
	TimeRange    TimeRange `json:"time_range"`
}

type TimeRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}
