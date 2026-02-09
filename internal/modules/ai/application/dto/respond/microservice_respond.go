package respond

type PredictRespond struct {
	Prediction string `json:"prediction"`
	CacheHit   bool   `json:"cache_hit"`
	TokensUsed int    `json:"tokens_used"`
	LatencyMs  int64  `json:"latency_ms"`
}

type PolishOption struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

type PolishRespond struct {
	Polishes   []PolishOption `json:"polishes"`
	CacheHit   bool           `json:"cache_hit"`
	TokensUsed int            `json:"tokens_used"`
	LatencyMs  int64          `json:"latency_ms"`
}

type DigestRespond struct {
	Summary    string   `json:"summary"`
	Topics     []string `json:"topics"`
	Mentions   []string `json:"mentions"`
	LatencyMs  int64    `json:"latency_ms"`
	CacheHit   bool     `json:"cache_hit"`
	TokensUsed int      `json:"tokens_used"`
}
