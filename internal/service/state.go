package service

import (
	"time"

	"github.com/peshk1n/site-monitor/internal/models"
)

func Advance(r *models.Runtime, m models.Monitor, result models.Result, now time.Time) *time.Time {
	if result.Outcome != "success" && result.Outcome != "warning" && result.Outcome != "failure" {
		r.ClearCandidate()
		return nil
	}
	observed := *result.ObservedAt
	if r.FreshUntil != nil && observed.After(*r.FreshUntil) {
		r.ClearCandidate()
	}
	r.LastCheckID = &result.ID
	r.LastObservedAt = &observed
	fresh := observed.Add(time.Duration(2*m.IntervalSeconds+m.TimeoutSeconds) * time.Second)
	r.FreshUntil = &fresh
	target := map[string]string{"success": "up", "warning": "degraded", "failure": "down"}[result.Outcome]
	if target == r.HealthStatus {
		r.ClearCandidate()
		return nil
	}
	if r.CandidateStatus == nil || *r.CandidateStatus != target {
		r.CandidateStatus = &target
		r.CandidateCount = 1
		r.CandidateSince = &observed
	} else {
		r.CandidateCount++
	}
	threshold := m.FailureThreshold
	rank := map[string]int{"unknown": -1, "up": 0, "degraded": 1, "down": 2}
	if r.HealthStatus == "unknown" && target == "up" {
		threshold = 1
	} else if rank[target] < rank[r.HealthStatus] {
		threshold = m.RecoveryThreshold
	}
	if r.CandidateCount < threshold {
		return nil
	}
	first := *r.CandidateSince
	r.HealthStatus = target
	r.StateChangedAt = now
	r.ClearCandidate()
	return &first
}
