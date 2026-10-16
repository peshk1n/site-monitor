package models

import "time"

type User struct {
	ID          int64  `db:"id" json:"id"`
	TelegramID  int64  `db:"telegram_user_id" json:"telegram_user_id"`
	DisplayName string `db:"display_name" json:"display_name"`
}

type Project struct {
	Summary    map[string]int64 `db:"-" json:"summary,omitempty"`
	ID         int64            `db:"id" json:"id"`
	Slug       string           `db:"slug" json:"slug"`
	Name       string           `db:"name" json:"name"`
	CreatedAt  time.Time        `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time        `db:"updated_at" json:"updated_at"`
	ArchivedAt *time.Time       `db:"archived_at" json:"archived_at"`
	Role       string           `db:"role" json:"role,omitempty"`
}

type MonitorConfig struct {
	Name                string  `db:"name" json:"name"`
	URL                 string  `db:"url" json:"url"`
	IntervalSeconds     int     `db:"interval_seconds" json:"interval_seconds"`
	TimeoutSeconds      int     `db:"timeout_seconds" json:"timeout_seconds"`
	ExpectedStatusCodes []int64 `db:"-" json:"expected_status_codes"`
	BodyMatchType       string  `db:"body_match_type" json:"body_match_type"`
	BodyPattern         *string `db:"body_pattern" json:"body_pattern"`
	WarningResponseMS   *int    `db:"warning_response_ms" json:"warning_response_ms"`
	FailureThreshold    int     `db:"failure_threshold" json:"failure_threshold"`
	RecoveryThreshold   int     `db:"recovery_threshold" json:"recovery_threshold"`
	MaxBodyBytes        int     `db:"max_body_bytes" json:"max_body_bytes"`
	MaxRedirects        int     `db:"max_redirects" json:"max_redirects"`
}

func DefaultMonitorConfig() MonitorConfig {
	return MonitorConfig{IntervalSeconds: 60, TimeoutSeconds: 10, ExpectedStatusCodes: []int64{200}, BodyMatchType: "none", FailureThreshold: 3, RecoveryThreshold: 2, MaxBodyBytes: 1048576, MaxRedirects: 5}
}

type Monitor struct {
	MonitorConfig
	ID            int64      `db:"id" json:"id"`
	ProjectID     int64      `db:"project_id" json:"project_id"`
	IsActive      bool       `db:"is_active" json:"is_active"`
	ConfigVersion int64      `db:"config_version" json:"config_version"`
	CreatedAt     time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at" json:"updated_at"`
	ArchivedAt    *time.Time `db:"archived_at" json:"archived_at"`
}

type Runtime struct {
	MonitorID            int64      `db:"monitor_id"`
	NextRunAt            time.Time  `db:"next_run_at"`
	ActiveRunID          *string    `db:"active_run_id"`
	LeaseOwner           *string    `db:"lease_owner"`
	LeaseExpiresAt       *time.Time `db:"lease_expires_at"`
	RunningConfigVersion *int64     `db:"running_config_version"`
	HealthStatus         string     `db:"health_status"`
	StateConfigVersion   int64      `db:"state_config_version"`
	LastCheckID          *int64     `db:"last_check_id"`
	LastObservedAt       *time.Time `db:"last_observed_at"`
	FreshUntil           *time.Time `db:"fresh_until"`
	StateChangedAt       time.Time  `db:"state_changed_at"`
	UpdatedAt            time.Time  `db:"updated_at"`
	EvaluationStartedAt  time.Time  `db:"evaluation_started_at"`
	CandidateStatus      *string    `db:"candidate_status"`
	CandidateCount       int        `db:"candidate_count"`
	CandidateSince       *time.Time `db:"candidate_since"`
}

func (r *Runtime) ClearCandidate() {
	r.CandidateStatus = nil
	r.CandidateCount = 0
	r.CandidateSince = nil
}
func (r *Runtime) Release() {
	r.ActiveRunID = nil
	r.LeaseOwner = nil
	r.LeaseExpiresAt = nil
	r.RunningConfigVersion = nil
}

type MonitorView struct {
	Monitor
	HealthStatus    string     `db:"health_status" json:"health_status"`
	Category        string     `db:"category" json:"category"`
	LastObservedAt  *time.Time `db:"last_observed_at" json:"last_observed_at"`
	FreshUntil      *time.Time `db:"fresh_until" json:"fresh_until"`
	CandidateStatus *string    `db:"candidate_status" json:"candidate_status"`
	CandidateCount  int        `db:"candidate_count" json:"candidate_count"`
	OpenIncidentID  *int64     `db:"open_incident_id" json:"open_incident_id"`
	LastOutcome     *string    `db:"last_outcome" json:"last_outcome"`
	LastResponseMS  *int       `db:"last_response_ms" json:"last_response_ms"`
}

type Result struct {
	ID            int64      `db:"id" json:"id"`
	MonitorID     int64      `db:"monitor_id" json:"monitor_id"`
	RunID         string     `db:"run_id" json:"run_id"`
	Origin        string     `db:"origin" json:"origin"`
	ConfigVersion *int64     `db:"config_version" json:"config_version"`
	ScheduledAt   *time.Time `db:"scheduled_at" json:"scheduled_at"`
	StartedAt     *time.Time `db:"started_at" json:"started_at"`
	FinishedAt    *time.Time `db:"finished_at" json:"finished_at"`
	ObservedAt    *time.Time `db:"observed_at" json:"observed_at"`
	RecordedAt    time.Time  `db:"recorded_at" json:"recorded_at"`
	Outcome       string     `db:"outcome" json:"outcome"`
	StatusCode    *int       `db:"status_code" json:"status_code"`
	ResponseMS    *int       `db:"response_ms" json:"response_ms"`
	ErrorKind     *string    `db:"error_kind" json:"error_kind"`
	ErrorMessage  *string    `db:"error_message" json:"error_message"`
	Accepted      bool       `db:"accepted" json:"accepted"`
	DiscardReason *string    `db:"discard_reason" json:"discard_reason"`
}

type Incident struct {
	ID              int64      `db:"id" json:"id"`
	ProjectID       int64      `db:"project_id" json:"project_id"`
	MonitorID       int64      `db:"monitor_id" json:"monitor_id"`
	Severity        string     `db:"severity" json:"severity"`
	FirstObservedAt time.Time  `db:"first_observed_at" json:"first_observed_at"`
	OpenedAt        time.Time  `db:"opened_at" json:"opened_at"`
	ResolvedAt      *time.Time `db:"resolved_at" json:"resolved_at"`
	Resolution      *string    `db:"resolution" json:"resolution"`
	Summary         string     `db:"summary" json:"summary"`
}

type Event struct {
	Incident         Incident
	Kind             string
	ConfigVersion    int64
	DeduplicationKey string
	OccurredAt       time.Time
}
type Job struct {
	Monitor     Monitor
	RunID       string
	ScheduledAt time.Time
}
type Page struct {
	Limit  int
	Offset int
}
type MonitorFilter struct {
	Archived bool
	Search   string
	Category string
	Page     Page
}
type Window struct {
	From time.Time
	To   time.Time
}
type ChartPoint struct {
	Time          time.Time `db:"time" json:"time"`
	Count         int64     `db:"count" json:"count"`
	Failures      int64     `db:"failures" json:"failures"`
	ResponseMS    float64   `db:"response_ms" json:"response_ms"`
	ConfigVersion int64     `db:"config_version" json:"config_version"`
}
type Dashboard struct {
	OpenIncidentsHasMore bool             `json:"open_incidents_has_more"`
	IncidentsHasMore     bool             `json:"incidents_has_more"`
	Project              Project          `json:"project"`
	AsOf                 time.Time        `json:"as_of"`
	From                 time.Time        `json:"from"`
	To                   time.Time        `json:"to"`
	Summary              map[string]int64 `json:"summary"`
	OpenIncidents        []Incident       `json:"open_incidents"`
	Incidents            []Incident       `json:"incidents"`
	Chart                []ChartPoint     `json:"chart"`
	ChartMonitorID       *int64           `json:"chart_monitor_id"`
}
