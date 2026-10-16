package probe

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/peshk1n/site-monitor/internal/models"
)

func TestHTTPOutcomes(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		body    string
		pattern string
		limit   int
		outcome string
		kind    string
	}{
		{"success", 200, "healthy", "", 100, "success", ""},
		{"unexpected code", 503, "healthy", "", 100, "failure", "http_status"},
		{"body mismatch", 200, "unhealthy", "^healthy$", 100, "failure", "body_mismatch"},
		{"body match", 200, "healthy", "^healthy$", 100, "success", ""},
		{"body limit", 200, "too long", "", 2, "failure", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); io.WriteString(w, tc.body) }))
			defer server.Close()
			p := New()
			defer p.Close()
			p.transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
			}
			m := models.Monitor{MonitorConfig: models.DefaultMonitorConfig()}
			m.URL = "http://example.com"
			m.MaxBodyBytes = tc.limit
			if tc.pattern != "" {
				m.BodyMatchType = "regexp"
				m.BodyPattern = &tc.pattern
			}
			result := p.Run(context.Background(), models.Job{Monitor: m})
			if result.Outcome != tc.outcome {
				t.Fatalf("outcome %s: %v", result.Outcome, result.ErrorMessage)
			}
			if tc.kind != "" && (result.ErrorKind == nil || *result.ErrorKind != tc.kind) {
				t.Fatalf("kind: %v", result.ErrorKind)
			}
			if result.FinishedAt.Before(*result.StartedAt) || result.ResponseMS == nil {
				t.Fatal("invalid timings")
			}
		})
	}
}
func TestRedirectCannotReachPrivateDestination(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "http://127.0.0.1/private", 302) }))
	defer server.Close()
	p := New()
	defer p.Close()
	calls := 0
	p.transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		calls++
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	m := models.Monitor{MonitorConfig: models.DefaultMonitorConfig()}
	m.URL = "http://example.com"
	result := p.Run(context.Background(), models.Job{Monitor: m})
	if result.Outcome != "error" || calls != 1 {
		t.Fatalf("unsafe redirect followed: %s / %d", result.Outcome, calls)
	}
}
func TestCancellation(t *testing.T) {
	p := New()
	defer p.Close()
	m := models.Monitor{MonitorConfig: models.DefaultMonitorConfig()}
	m.URL = "https://example.com"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := p.Run(ctx, models.Job{Monitor: m})
	if result.Outcome != "cancelled" {
		t.Fatalf("cancelled check reported %s", result.Outcome)
	}
}
func TestWarningIncludesBodyReadTime(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		time.Sleep(20 * time.Millisecond)
		io.Copy(w, strings.NewReader("ok"))
	}))
	defer server.Close()
	p := New()
	defer p.Close()
	p.transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	m := models.Monitor{MonitorConfig: models.DefaultMonitorConfig()}
	m.URL = "http://example.com"
	threshold := 1
	m.WarningResponseMS = &threshold
	if result := p.Run(context.Background(), models.Job{Monitor: m}); result.Outcome != "warning" {
		t.Fatalf("body time not counted: %+v", result)
	}
}

func TestRedirectLimitAndTimeout(t *testing.T) {
	t.Run("redirect limit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "http://example.com/again", 302) }))
		defer server.Close()
		p := New()
		defer p.Close()
		p.transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
		}
		m := models.Monitor{MonitorConfig: models.DefaultMonitorConfig()}
		m.URL = "http://example.com"
		m.MaxRedirects = 1
		result := p.Run(context.Background(), models.Job{Monitor: m})
		if result.Outcome != "failure" || result.ErrorMessage == nil || *result.ErrorMessage != "redirect limit exceeded" {
			t.Fatalf("wrong redirect result: %+v", result)
		}
	})
	t.Run("timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
		defer server.Close()
		p := New()
		defer p.Close()
		p.transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
		}
		m := models.Monitor{MonitorConfig: models.DefaultMonitorConfig()}
		m.URL = "http://example.com"
		m.TimeoutSeconds = 1
		result := p.Run(context.Background(), models.Job{Monitor: m})
		if result.Outcome != "failure" || result.ErrorKind == nil || *result.ErrorKind != "timeout" {
			t.Fatalf("wrong timeout result: %+v", result)
		}
	})
}
