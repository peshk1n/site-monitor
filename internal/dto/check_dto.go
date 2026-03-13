package dto

type CheckResponse struct {
	ID         int    `json:"id"`
	MonitorID  int    `json:"monitor_id"`
	StatusCode int    `json:"status_code"`
	ResponseMs int    `json:"response_ms"`
	IsUp       bool   `json:"is_up"`
	Error      string `json:"error,omitempty"`
	CheckedAt  string `json:"checked_at"`
}

type CheckListResponse struct {
	Checks []CheckResponse `json:"checks"`
}
