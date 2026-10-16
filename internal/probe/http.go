package probe

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/peshk1n/site-monitor/internal/models"
	"github.com/peshk1n/site-monitor/internal/target"
)

type HTTP struct{ transport *http.Transport }

var errRedirectLimit = errors.New("redirect limit exceeded")

func New() *HTTP {
	return &HTTP{transport: &http.Transport{DialContext: target.DialPublic(net.DefaultResolver), TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}, TLSHandshakeTimeout: 10 * time.Second, IdleConnTimeout: 30 * time.Second, MaxIdleConns: 100, MaxIdleConnsPerHost: 2, MaxResponseHeaderBytes: 1 << 20}}
}
func (p *HTTP) Close() { p.transport.CloseIdleConnections() }
func (p *HTTP) Run(parent context.Context, job models.Job) models.Result {
	m := job.Monitor
	start := time.Now().UTC()
	c := models.Result{StartedAt: &start, Outcome: "success"}
	ctx, cancel := context.WithTimeout(parent, time.Duration(m.TimeoutSeconds)*time.Second)
	defer cancel()
	fail := func(kind, message string) {
		c.Outcome = "failure"
		if kind != "" {
			c.ErrorKind = &kind
		}
		c.ErrorMessage = &message
	}
	finish := func() models.Result {
		end := time.Now().UTC()
		duration := int(end.Sub(start).Milliseconds())
		c.FinishedAt = &end
		c.ObservedAt = &end
		c.ResponseMS = &duration
		return c
	}
	if err := target.ValidateURL(m.URL); err != nil {
		c.Outcome = "error"
		kind := "execution"
		message := "destination rejected by network policy"
		c.ErrorKind = &kind
		c.ErrorMessage = &message
		return finish()
	}
	client := &http.Client{Transport: p.transport, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > m.MaxRedirects {
			return errRedirectLimit
		}
		if err := target.ValidateURL(req.URL.String()); err != nil {
			return err
		}
		if len(via) > 0 && via[0].URL.Scheme == "https" && req.URL.Scheme != "https" {
			return target.ErrUnsafe
		}
		return nil
	}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.URL, nil)
	if err != nil {
		fail("execution", "invalid request")
		c.Outcome = "error"
		return finish()
	}
	req.Header.Set("User-Agent", "SiteMonitor/1.0")
	resp, err := client.Do(req)
	if err != nil {
		if resp != nil {
			c.StatusCode = &resp.StatusCode
		}
		kind, message := "connection", "connection failed"
		var dns *net.DNSError
		var tlsErr *tls.CertificateVerificationError
		switch {
		case parent.Err() != nil:
			kind = "cancelled"
			message = "check cancelled"
		case errors.Is(err, errRedirectLimit):
			kind = ""
			message = "redirect limit exceeded"
		case errors.Is(err, context.DeadlineExceeded):
			kind = "timeout"
			message = "request timed out"
		case errors.As(err, &dns):
			kind = "dns"
			message = "DNS lookup failed"
		case errors.As(err, &tlsErr):
			kind = "tls"
			message = "TLS verification failed"
		case errors.Is(err, target.ErrUnsafe):
			kind = "execution"
			message = "destination rejected by network policy"
		}
		fail(kind, message)
		if kind == "cancelled" {
			c.Outcome = "cancelled"
		}
		if kind == "execution" {
			c.Outcome = "error"
		}
		return finish()
	}
	defer resp.Body.Close()
	c.StatusCode = &resp.StatusCode
	body, err := io.ReadAll(io.LimitReader(resp.Body, int64(m.MaxBodyBytes)+1))
	if err != nil {
		if parent.Err() != nil {
			fail("cancelled", "check cancelled")
			c.Outcome = "cancelled"
		} else if ctx.Err() != nil {
			fail("timeout", "response body timed out")
		} else {
			fail("connection", "response body read failed")
		}
		return finish()
	}
	if len(body) > m.MaxBodyBytes {
		fail("", "response body limit exceeded")
		return finish()
	}
	if !slices.Contains(m.ExpectedStatusCodes, int64(resp.StatusCode)) {
		fail("http_status", "unexpected HTTP status")
		return finish()
	}
	if m.BodyMatchType != "none" {
		if m.BodyPattern == nil {
			fail("execution", "missing body pattern")
			c.Outcome = "error"
			return finish()
		}
		matched := strings.Contains(string(body), *m.BodyPattern)
		if m.BodyMatchType == "regexp" {
			pattern, err := regexp.Compile(*m.BodyPattern)
			if err != nil {
				fail("execution", "invalid body pattern")
				c.Outcome = "error"
				return finish()
			}
			matched = pattern.Match(body)
		}
		if !matched {
			fail("body_mismatch", "response body does not match")
			return finish()
		}
	}
	c = finish()
	if m.WarningResponseMS != nil && *c.ResponseMS > *m.WarningResponseMS {
		c.Outcome = "warning"
	}
	return c
}
