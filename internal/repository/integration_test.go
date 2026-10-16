package repository_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/repository"
	"github.com/peshk1n/site-monitor/internal/service"
	"github.com/peshk1n/site-monitor/internal/testutil"
)

type fixture struct {
	db      *sqlx.DB
	app     *service.Application
	owner   models.User
	other   models.User
	project models.Project
	monitor models.Monitor
}

func setup(t *testing.T) fixture {
	t.Helper()
	db := testutil.Database(t)
	app := service.New(repository.New(db))
	ctx := context.Background()
	owner, err := app.Identify(ctx, 1099511627776, "Owner")
	must(t, err)
	other, err := app.Identify(ctx, 1099511627777, "Other")
	must(t, err)
	p, err := app.CreateProject(ctx, owner.ID, "Main")
	must(t, err)
	c := models.DefaultMonitorConfig()
	c.Name = "Health"
	c.URL = "https://example.com/health"
	m, err := app.CreateMonitor(ctx, owner.ID, p.ID, c)
	must(t, err)
	return fixture{db, app, owner, other, p, m}
}
func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func (f fixture) scalar(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	must(t, f.db.Get(&n, sql, args...))
	return n
}
func (f fixture) claim(t *testing.T) models.Job {
	t.Helper()
	_, err := f.db.Exec("UPDATE monitor_runtime SET next_run_at=CURRENT_TIMESTAMP WHERE monitor_id=$1", f.monitor.ID)
	must(t, err)
	job, err := f.app.Claim(context.Background(), "test")
	must(t, err)
	if job == nil {
		t.Fatal("no job available")
	}
	return *job
}
func observation(outcome string) models.Result {
	now := time.Now().UTC()
	start := now.Add(-time.Millisecond)
	ms, code := 1, 200
	return models.Result{StartedAt: &start, FinishedAt: &now, ObservedAt: &now, ResponseMS: &ms, StatusCode: &code, Outcome: outcome}
}
func (f fixture) finish(t *testing.T, outcome string) {
	t.Helper()
	job := f.claim(t)
	must(t, f.app.Complete(context.Background(), job, observation(outcome)))
}
func (f fixture) view(t *testing.T) models.MonitorView {
	t.Helper()
	m, err := f.app.Monitor(context.Background(), f.owner.ID, f.project.ID, f.monitor.ID)
	must(t, err)
	return m
}

func TestProjectIsolationAndRoles(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	page := models.Page{Limit: 20}
	projects, err := f.app.Projects(ctx, f.owner.ID, false, page)
	must(t, err)
	if len(projects) != 1 || projects[0].Role != "owner" {
		t.Fatal("owner membership missing")
	}
	for _, id := range []int64{f.project.ID, 99999} {
		if _, err = f.app.Project(ctx, f.other.ID, id); !errors.Is(err, service.ErrNotFound) {
			t.Fatalf("project %d leaked: %v", id, err)
		}
	}
	if _, err = f.app.Monitor(ctx, f.other.ID, f.project.ID, f.monitor.ID); !errors.Is(err, service.ErrNotFound) {
		t.Fatal("monitor leaked")
	}
	_, err = f.db.Exec("INSERT INTO project_members (project_id,user_id,role) VALUES ($1,$2,'viewer')", f.project.ID, f.other.ID)
	must(t, err)
	_, err = f.app.Monitor(ctx, f.other.ID, f.project.ID, f.monitor.ID)
	must(t, err)
	if _, err = f.app.ChangeMonitor(ctx, f.other.ID, f.project.ID, f.monitor.ID, 1, "pause"); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("viewer mutated monitor: %v", err)
	}
	_, err = f.db.Exec("UPDATE project_members SET role='editor' WHERE user_id=$1", f.other.ID)
	must(t, err)
	_, err = f.app.ChangeMonitor(ctx, f.other.ID, f.project.ID, f.monitor.ID, 1, "pause")
	must(t, err)
	if _, err = f.app.ChangeProject(ctx, f.other.ID, f.project.ID, "archive", ""); !errors.Is(err, service.ErrForbidden) {
		t.Fatal("editor archived project")
	}
	_, err = f.db.Exec("UPDATE users SET archived_at=CURRENT_TIMESTAMP WHERE id=$1", f.owner.ID)
	must(t, err)
	if _, err = f.app.Identify(ctx, f.owner.TelegramID, "Owner"); !errors.Is(err, service.ErrUnauthorized) {
		t.Fatal("archived user authenticated")
	}
}

func TestIncidentLifecycleAndOutbox(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	var channel, rule int64
	must(t, f.db.Get(&channel, "INSERT INTO notification_channels (project_id,name,kind,destination) VALUES ($1,'Owner','telegram','123') RETURNING id", f.project.ID))
	must(t, f.db.Get(&rule, "INSERT INTO notification_rules (project_id,name) VALUES ($1,'All') RETURNING id", f.project.ID))
	_, err := f.db.Exec("INSERT INTO notification_rule_channels (rule_id,project_id,channel_id) VALUES ($1,$2,$3)", rule, f.project.ID, channel)
	must(t, err)
	f.finish(t, "success")
	for i := 0; i < 3; i++ {
		f.finish(t, "warning")
	}
	m := f.view(t)
	if m.HealthStatus != "degraded" || m.OpenIncidentID == nil {
		t.Fatalf("warning not opened: %+v", m)
	}
	incident := *m.OpenIncidentID
	for i := 0; i < 3; i++ {
		f.finish(t, "failure")
	}
	m = f.view(t)
	if m.HealthStatus != "down" || *m.OpenIncidentID != incident {
		t.Fatal("severity change created another incident")
	}
	for i := 0; i < 2; i++ {
		f.finish(t, "warning")
	}
	for i := 0; i < 2; i++ {
		f.finish(t, "success")
	}
	m = f.view(t)
	if m.HealthStatus != "up" || m.OpenIncidentID != nil {
		t.Fatal("recovery not confirmed")
	}
	if f.scalar(t, "SELECT count(*) FROM incidents") != 1 || f.scalar(t, "SELECT count(*) FROM monitor_events") != 4 || f.scalar(t, "SELECT count(*) FROM notification_deliveries") != 4 {
		t.Fatal("unexpected incident/event/delivery count")
	}
	d, err := f.app.Dashboard(ctx, f.owner.ID, f.project.ID, &f.monitor.ID, models.Window{From: time.Now().Add(-time.Hour), To: time.Now()})
	must(t, err)
	if d.Summary["up"] != 1 || len(d.Incidents) != 1 || len(d.Chart) == 0 {
		t.Fatalf("invalid dashboard: %+v", d)
	}
	if _, err = f.app.Dashboard(ctx, f.other.ID, f.project.ID, nil, models.Window{From: time.Now().Add(-time.Hour), To: time.Now()}); !errors.Is(err, service.ErrNotFound) {
		t.Fatal("dashboard leaked")
	}
}

func TestPauseEditArchiveRejectLateResults(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		f.finish(t, "failure")
	}
	job := f.claim(t)
	m, err := f.app.ChangeMonitor(ctx, f.owner.ID, f.project.ID, f.monitor.ID, 1, "pause")
	must(t, err)
	must(t, f.app.Complete(ctx, job, observation("success")))
	must(t, f.app.Complete(ctx, job, observation("success")))
	if f.scalar(t, "SELECT count(*) FROM check_results WHERE NOT accepted") != 1 || f.view(t).HealthStatus != "down" {
		t.Fatal("late/duplicate result changed state")
	}
	m, err = f.app.ChangeMonitor(ctx, f.owner.ID, f.project.ID, m.ID, m.ConfigVersion, "resume")
	must(t, err)
	f.finish(t, "success")
	if f.view(t).OpenIncidentID == nil {
		t.Fatal("recovered on first success")
	}
	f.finish(t, "success")
	config := m.MonitorConfig
	config.Name = "Renamed"
	renamed, err := f.app.UpdateMonitor(ctx, f.owner.ID, f.project.ID, m.ID, m.ConfigVersion, config)
	must(t, err)
	if renamed.ConfigVersion != m.ConfigVersion || f.view(t).HealthStatus != "up" {
		t.Fatal("rename reset evaluation")
	}
	for i := 0; i < 3; i++ {
		f.finish(t, "failure")
	}
	job = f.claim(t)
	config.URL = "https://example.com/changed"
	m, err = f.app.UpdateMonitor(ctx, f.owner.ID, f.project.ID, m.ID, m.ConfigVersion, config)
	must(t, err)
	must(t, f.app.Complete(ctx, job, observation("success")))
	if f.view(t).HealthStatus != "unknown" || f.scalar(t, "SELECT count(*) FROM incidents WHERE resolution='reconfigured'") != 1 {
		t.Fatal("configuration reset incorrect")
	}
	if _, err = f.app.UpdateMonitor(ctx, f.owner.ID, f.project.ID, m.ID, m.ConfigVersion-1, config); !errors.Is(err, service.ErrConflict) {
		t.Fatal("stale update accepted")
	}
	for i := 0; i < 3; i++ {
		f.finish(t, "failure")
	}
	m, err = f.app.ChangeMonitor(ctx, f.owner.ID, f.project.ID, m.ID, m.ConfigVersion, "archive")
	must(t, err)
	if f.scalar(t, "SELECT count(*) FROM incidents WHERE resolution='archived'") != 1 {
		t.Fatal("archive did not close incident")
	}
	if job, err := f.app.Claim(ctx, "test"); err != nil || job != nil {
		t.Fatal("archived monitor claimed")
	}
	m, err = f.app.ChangeMonitor(ctx, f.owner.ID, f.project.ID, m.ID, m.ConfigVersion, "restore")
	must(t, err)
	if m.IsActive || m.ArchivedAt != nil {
		t.Fatal("restore enabled monitor")
	}
}

func TestProjectArchivePreservesIndividualArchive(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	c := f.monitor.MonitorConfig
	c.Name = "Other"
	second, err := f.app.CreateMonitor(ctx, f.owner.ID, f.project.ID, c)
	must(t, err)
	_, err = f.app.ChangeMonitor(ctx, f.owner.ID, f.project.ID, second.ID, 1, "archive")
	must(t, err)
	for i := 0; i < 3; i++ {
		f.finish(t, "failure")
	}
	job := f.claim(t)
	_, err = f.app.ChangeProject(ctx, f.owner.ID, f.project.ID, "archive", "")
	must(t, err)
	must(t, f.app.Complete(ctx, job, observation("success")))
	if _, err = f.app.CreateMonitor(ctx, f.owner.ID, f.project.ID, c); !errors.Is(err, service.ErrConflict) {
		t.Fatal("monitor created in archived project")
	}
	if f.scalar(t, "SELECT count(*) FROM incidents WHERE resolved_at IS NULL") != 0 {
		t.Fatal("project incident remained open")
	}
	_, err = f.app.ChangeProject(ctx, f.owner.ID, f.project.ID, "restore", "")
	must(t, err)
	if f.scalar(t, "SELECT count(*) FROM monitors WHERE is_active") != 0 || f.scalar(t, "SELECT count(*) FROM monitors WHERE archived_at IS NOT NULL") != 1 {
		t.Fatal("project restoration lost monitor modes")
	}
}

func TestTransactionRollbackOnAuditAndDeliveryFailure(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	_, err := f.db.Exec(`CREATE FUNCTION reject_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test failure'; END $$;
	CREATE TRIGGER reject_audit BEFORE INSERT ON audit_log FOR EACH ROW EXECUTE FUNCTION reject_write()`)
	must(t, err)
	before := f.scalar(t, "SELECT count(*) FROM projects")
	if _, err = f.app.CreateProject(ctx, f.owner.ID, "Rollback"); err == nil {
		t.Fatal("expected failure")
	}
	if f.scalar(t, "SELECT count(*) FROM projects") != before {
		t.Fatal("orphaned project after audit failure")
	}
	_, err = f.db.Exec("DROP TRIGGER reject_audit ON audit_log")
	must(t, err)
	var channel, rule int64
	must(t, f.db.Get(&channel, "INSERT INTO notification_channels (project_id,name,kind,destination) VALUES ($1,'Owner','telegram','123') RETURNING id", f.project.ID))
	must(t, f.db.Get(&rule, "INSERT INTO notification_rules (project_id,name) VALUES ($1,'All') RETURNING id", f.project.ID))
	_, err = f.db.Exec("INSERT INTO notification_rule_channels (rule_id,project_id,channel_id) VALUES ($1,$2,$3)", rule, f.project.ID, channel)
	must(t, err)
	_, err = f.db.Exec("CREATE TRIGGER reject_delivery BEFORE INSERT ON notification_deliveries FOR EACH ROW EXECUTE FUNCTION reject_write()")
	must(t, err)
	f.finish(t, "failure")
	f.finish(t, "failure")
	job := f.claim(t)
	result := observation("failure")
	if err = f.app.Complete(ctx, job, result); err == nil {
		t.Fatal("expected failed delivery transaction")
	}
	if f.scalar(t, "SELECT count(*) FROM check_results") != 2 || f.scalar(t, "SELECT count(*) FROM incidents") != 0 || f.view(t).CandidateCount != 2 {
		t.Fatal("partial transaction committed")
	}
	_, err = f.db.Exec("DROP TRIGGER reject_delivery ON notification_deliveries")
	must(t, err)
	must(t, f.app.Complete(ctx, job, result))
	if f.view(t).HealthStatus != "down" {
		t.Fatal("retry failed")
	}
}

func TestConcurrentClaimsAndExpiredLease(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_, err := f.app.CreateMonitor(ctx, f.owner.ID, f.project.ID, f.monitor.MonitorConfig)
		must(t, err)
	}
	var wg sync.WaitGroup
	results := make(chan *models.Job, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); job, err := f.app.Claim(ctx, "parallel"); results <- job; errs <- err }()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		must(t, err)
	}
	ids := map[int64]models.Job{}
	for job := range results {
		if job == nil {
			continue
		}
		if _, ok := ids[job.Monitor.ID]; ok {
			t.Fatal("duplicate monitor claim")
		}
		ids[job.Monitor.ID] = *job
	}
	if len(ids) != 4 {
		t.Fatalf("claimed %d monitors", len(ids))
	}
	old := ids[f.monitor.ID]
	_, err := f.db.Exec("UPDATE monitor_runtime SET lease_expires_at=CURRENT_TIMESTAMP-INTERVAL '1 second' WHERE monitor_id=$1", f.monitor.ID)
	must(t, err)
	newJob, err := f.app.Claim(ctx, "replacement")
	must(t, err)
	if newJob == nil || newJob.RunID == old.RunID {
		t.Fatal("expired lease not replaced")
	}
	must(t, f.app.Complete(ctx, old, observation("failure")))
	must(t, f.app.Complete(ctx, *newJob, observation("success")))
	if f.scalar(t, "SELECT count(*) FROM check_results WHERE NOT accepted") != 1 || f.view(t).HealthStatus != "up" {
		t.Fatal("lease fencing failed")
	}
}

func TestFreshnessAndConcurrentEdits(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	f.finish(t, "success")
	_, err := f.db.Exec("UPDATE monitor_runtime SET fresh_until=last_observed_at, last_observed_at=last_observed_at-INTERVAL '10 minutes' WHERE monitor_id=$1", f.monitor.ID)
	must(t, err)
	if f.view(t).Category != "stale" || f.view(t).HealthStatus != "up" {
		t.Fatal("freshness changed service health")
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, address := range []string{"https://example.com/one", "https://example.com/two"} {
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			c := f.monitor.MonitorConfig
			c.URL = address
			_, err := f.app.UpdateMonitor(ctx, f.owner.ID, f.project.ID, f.monitor.ID, 1, c)
			errs <- err
		}(address)
	}
	wg.Wait()
	close(errs)
	success, conflicts := 0, 0
	for err := range errs {
		if err == nil {
			success++
		} else if errors.Is(err, service.ErrConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("lost concurrent edit: %d successes, %d conflicts", success, conflicts)
	}
}

func TestCreationRequestDeduplication(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	key := "11111111-1111-4111-8111-111111111111"
	var wg sync.WaitGroup
	results := make(chan models.Project, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := f.app.CreateProject(ctx, f.owner.ID, "Once", key)
			results <- p
			errs <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		must(t, err)
	}
	var id int64
	for p := range results {
		if id != 0 && p.ID != id {
			t.Fatal("duplicate project created")
		}
		id = p.ID
	}
	if f.scalar(t, "SELECT count(*) FROM projects WHERE name='Once'") != 1 || f.scalar(t, "SELECT count(*) FROM creation_requests") != 1 {
		t.Fatal("creation was not atomic")
	}
	if _, err := f.app.CreateProject(ctx, f.owner.ID, "Different", key); !errors.Is(err, service.ErrConflict) {
		t.Fatal("key reused for different input")
	}
	other, err := f.app.CreateProject(ctx, f.other.ID, "Once", key)
	must(t, err)
	if other.ID == id {
		t.Fatal("creation response leaked across users")
	}
	monitorKey := "22222222-2222-4222-8222-222222222222"
	m1, err := f.app.CreateMonitor(ctx, f.owner.ID, f.project.ID, f.monitor.MonitorConfig, monitorKey)
	must(t, err)
	m2, err := f.app.CreateMonitor(ctx, f.owner.ID, f.project.ID, f.monitor.MonitorConfig, monitorKey)
	must(t, err)
	if m1.ID != m2.ID {
		t.Fatal("duplicate monitor created")
	}
	if _, err = f.app.CreateProject(ctx, f.owner.ID, "Once", monitorKey); !errors.Is(err, service.ErrConflict) {
		t.Fatal("key reused across operations")
	}
	_, err = f.db.Exec(`CREATE FUNCTION reject_request_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test failure'; END $$;
	CREATE TRIGGER reject_audit BEFORE INSERT ON audit_log FOR EACH ROW EXECUTE FUNCTION reject_request_audit()`)
	must(t, err)
	failedKey := "33333333-3333-4333-8333-333333333333"
	if _, err = f.app.CreateProject(ctx, f.owner.ID, "Failed", failedKey); err == nil {
		t.Fatal("expected transaction failure")
	}
	if f.scalar(t, "SELECT count(*) FROM creation_requests WHERE request_key=$1", failedKey) != 0 {
		t.Fatal("failed operation left a replay record")
	}
}
