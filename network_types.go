package mindp

type RequestEventJS struct {
	URL     string         `json:"url"`
	Method  string         `json:"method"`
	Headers map[string]any `json:"headers"`
}
