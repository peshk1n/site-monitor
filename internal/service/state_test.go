package service

import (
	"testing"
	"time"

	"github.com/peshk1n/site-monitor/internal/models"
)

func TestStateTransitions(t *testing.T) {
	cases := []struct {
		name     string
		sequence []string
		states   []string
	}{
		{"failure and recovery", []string{"success", "failure", "failure", "failure", "success", "success"}, []string{"up", "up", "up", "down", "down", "up"}},
		{"severity changes", []string{"warning", "warning", "warning", "failure", "failure", "failure", "warning", "warning", "success", "success"}, []string{"unknown", "unknown", "degraded", "degraded", "degraded", "down", "down", "degraded", "degraded", "up"}},
		{"mixed results", []string{"success", "warning", "failure", "warning", "failure", "success"}, []string{"up", "up", "up", "up", "up", "up"}},
		{"execution error interrupts", []string{"failure", "failure", "error", "failure", "failure", "failure"}, []string{"unknown", "unknown", "unknown", "unknown", "unknown", "down"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := models.Monitor{MonitorConfig: models.DefaultMonitorConfig()}
			r := models.Runtime{HealthStatus: "unknown"}
			start := time.Now().UTC()
			for index, outcome := range tc.sequence {
				at := start.Add(time.Duration(index) * time.Minute)
				result := models.Result{ID: int64(index + 1), Outcome: outcome, ObservedAt: &at}
				first := Advance(&r, m, result, at)
				if r.HealthStatus != tc.states[index] {
					t.Fatalf("step %d: %s, want %s", index, r.HealthStatus, tc.states[index])
				}
				if first != nil && r.CandidateCount != 0 {
					t.Fatal("candidate not cleared after transition")
				}
				if tc.name == "failure and recovery" && index == 3 && !first.Equal(start.Add(time.Minute)) {
					t.Fatal("incident lost first observation")
				}
			}
		})
	}
}
func TestStaleGapRestartsConfirmation(t *testing.T) {
	m := models.Monitor{MonitorConfig: models.DefaultMonitorConfig()}
	r := models.Runtime{HealthStatus: "up"}
	now := time.Now().UTC()
	for _, seconds := range []int{0, 60, 300} {
		at := now.Add(time.Duration(seconds) * time.Second)
		Advance(&r, m, models.Result{Outcome: "failure", ObservedAt: &at}, at)
	}
	if r.CandidateCount != 1 || r.HealthStatus != "up" {
		t.Fatalf("gap carried confirmation: %+v", r)
	}
}
func TestValidation(t *testing.T) {
	for _, change := range []func(*models.MonitorConfig){
		func(c *models.MonitorConfig) { c.URL = "http://127.0.0.1" }, func(c *models.MonitorConfig) { c.TimeoutSeconds = 0 },
		func(c *models.MonitorConfig) { c.ExpectedStatusCodes = []int64{} }, func(c *models.MonitorConfig) { c.FailureThreshold = 101 },
		func(c *models.MonitorConfig) { p := "["; c.BodyMatchType = "regexp"; c.BodyPattern = &p },
	} {
		c := models.DefaultMonitorConfig()
		c.Name = "Test"
		c.URL = "https://example.com"
		change(&c)
		if ValidateConfig(&c) == nil {
			t.Fatalf("accepted invalid config: %+v", c)
		}
	}
}
