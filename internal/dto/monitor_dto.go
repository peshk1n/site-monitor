package dto

type CreateMonitorRequest struct {
	URL      string `json:"url"`
	Interval int    `json:"interval"`
	Timeout  int    `json:"timeout"`
}

type MonitorResponse struct {
	ID        int    `json:"id"`
	URL       string `json:"url"`
	Interval  int    `json:"interval"`
	Timeout   int    `json:"timeout"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
}

type MonitorListResponse struct {
	Monitors []MonitorResponse `json:"monitors"`
}

type UpdateMonitorRequest struct {
	Interval *int  `json:"interval"`
	Timeout  *int  `json:"timeout"`
	IsActive *bool `json:"is_active"`
}
