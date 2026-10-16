package handler_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/peshk1n/site-monitor/internal/auth"
	"github.com/peshk1n/site-monitor/internal/handler"
	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/repository"
	"github.com/peshk1n/site-monitor/internal/router"
	"github.com/peshk1n/site-monitor/internal/service"
	"github.com/peshk1n/site-monitor/internal/testutil"
)

func token(id int64) string {
	date := strconv.FormatInt(time.Now().Unix(), 10)
	user := fmt.Sprintf(`{"id":%d,"first_name":"Owner"}`, id)
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte("test-token"))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte("auth_date=" + date + "\nuser=" + user))
	return "tma " + url.Values{"auth_date": {date}, "user": {user}, "hash": {hex.EncodeToString(mac.Sum(nil))}}.Encode()
}
func TestAuthenticatedAPI(t *testing.T) {
	db := testutil.Database(t)
	app := service.New(repository.New(db))
	h := handler.New(app, auth.NewTelegram("test-token", app, time.Hour), slog.New(slog.NewTextHandler(io.Discard, nil)))
	r := router.New(h, db.PingContext)
	request := func(method, path, body, authorization string, want int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", authorization)
		var key string
		if err := db.Get(&key, "SELECT gen_random_uuid()"); err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Idempotency-Key", key)
		response := httptest.NewRecorder()
		r.ServeHTTP(response, req)
		if response.Code != want {
			t.Fatalf("%s %s: %d instead of %d: %s", method, path, response.Code, want, response.Body.String())
		}
		return response
	}
	request("GET", "/api/v1/projects", "", "", 401)
	request("GET", "/healthz", "", "", 200)
	request("GET", "/readyz", "", "", 200)
	owner, other := token(123456), token(654321)
	request("POST", "/api/v1/projects", `{"name":""}`, owner, 400)
	response := request("POST", "/api/v1/projects", `{"name":"Main"}`, owner, 201)
	var project models.Project
	if err := json.Unmarshal(response.Body.Bytes(), &project); err != nil {
		t.Fatal(err)
	}
	prefix := fmt.Sprintf("/api/v1/projects/%d", project.ID)
	request("GET", prefix, "", other, 404)
	request("GET", "/api/v1/projects/99999", "", other, 404)
	request("GET", "/api/v1/projects?limit=abc", "", owner, 400)
	request("POST", prefix+"/monitors", `{"name":"Unsafe","url":"http://127.0.0.1"}`, owner, 400)
	request("POST", prefix+"/monitors", `{"name":"Bad","url":"https://example.com","unexpected":true}`, owner, 400)
	request("POST", prefix+"/monitors", `{"name":"Bad","url":"https://example.com"} {}`, owner, 400)
	response = request("POST", prefix+"/monitors", `{"name":"Health","url":"https://example.com"}`, owner, 201)
	var monitor models.Monitor
	if err := json.Unmarshal(response.Body.Bytes(), &monitor); err != nil {
		t.Fatal(err)
	}
	if monitor.IntervalSeconds != 60 || monitor.FailureThreshold != 3 {
		t.Fatal("defaults not applied")
	}
	path := fmt.Sprintf("%s/monitors/%d", prefix, monitor.ID)
	request("GET", path, "", owner, 200)
	request("GET", path, "", other, 404)
	request("GET", path+"/results", "", owner, 200)
	request("POST", path+"/pause", `{"config_version":1}`, owner, 200)
	request("POST", path+"/resume", `{"config_version":1}`, owner, 409)
	request("POST", path+"/resume", `{"config_version":2}`, owner, 200)
	request("GET", prefix+"/dashboard?period=7d&monitor_id="+strconv.FormatInt(monitor.ID, 10), "", owner, 200)
	request("GET", prefix+"/dashboard?period=forever", "", owner, 400)
	request("POST", prefix+"/archive", "", owner, 200)
	request("POST", prefix+"/monitors", `{"name":"Blocked","url":"https://example.com"}`, owner, 409)
	request("GET", path+"/results", "", owner, 200)
	request("POST", prefix+"/restore", "", owner, 200)
	var active bool
	if err := db.GetContext(context.Background(), &active, "SELECT is_active FROM monitors WHERE id=$1", monitor.ID); err != nil {
		t.Fatal(err)
	}
	if active {
		t.Fatal("restoration resumed monitoring")
	}
}

func TestAuthenticationFailureIsJSON(t *testing.T) {
	h := handler.New(nil, auth.NewTelegram("", nil, time.Minute), slog.New(slog.NewTextHandler(io.Discard, nil)))
	r := router.New(h, func(context.Context) error { return nil })
	response := httptest.NewRecorder()
	r.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil))
	if response.Code != 401 || response.Header().Get("Content-Type") != "application/json" {
		t.Fatal("invalid auth failure response")
	}
}
