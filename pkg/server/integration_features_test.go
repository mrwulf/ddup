package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/italypaleale/ddup/pkg/healthcheck"
)

// These tests exercise the features of the open pull requests together:
// the build header on every API response, and the endpoint fields of the status API

// mixedStatusProvider returns a domain with an IP, a looked-up address and a proxied CNAME
type mixedStatusProvider struct {
	forceErr error
}

func (p *mixedStatusProvider) status() healthcheck.DomainStatus {
	since := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	failure := since.Add(time.Minute)
	return healthcheck.DomainStatus{
		Provider: "cf",
		Endpoints: []healthcheck.DomainStatusEndpoint{
			{Healthy: false, IP: "192.0.2.10", Type: "A", FailureCount: 2, Since: &failure, LastFailure: &failure},
			{Healthy: false, IP: "2001:db8::1", Type: "AAAA", FailureCount: 2, Since: &failure, LastFailure: &failure},
			{Healthy: true, IP: "0a1b2c3d.cfargotunnel.com", Type: "CNAME", Proxied: true, Since: &since, SinceStart: true},
		},
	}
}

func (p *mixedStatusProvider) GetAllDomainsStatus() map[string]healthcheck.DomainStatus {
	return map[string]healthcheck.DomainStatus{"app.example.com": p.status()}
}

func (p *mixedStatusProvider) GetDomainStatus(domain string) *healthcheck.DomainStatus {
	if domain != "app.example.com" {
		return nil
	}
	s := p.status()
	return &s
}

func (p *mixedStatusProvider) ForceCheck(ctx context.Context) error {
	return p.forceErr
}

func TestIntegrationBuildHeaderOnEveryAPIResponse(t *testing.T) {
	newServer := func(t *testing.T, p *mixedStatusProvider) *Server {
		t.Helper()
		s, err := NewServer(NewServerOpts{HealthChecker: p})
		require.NoError(t, err)
		return s
	}
	do := func(s *Server, method, path string, headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(t.Context(), method, path, nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		s.handler.ServeHTTP(rec, req)
		return rec
	}
	expected := newBuildInfo().Build
	require.NotEmpty(t, expected)

	cases := []struct {
		name     string
		provider *mixedStatusProvider
		method   string
		path     string
		headers  map[string]string
		status   int
	}{
		{name: "all statuses", method: http.MethodGet, path: "/api/status", status: http.StatusOK},
		{name: "one status", method: http.MethodGet, path: "/api/status/app.example.com", status: http.StatusOK},
		{name: "unknown domain", method: http.MethodGet, path: "/api/status/missing.example.com", status: http.StatusNotFound},
		{name: "check", method: http.MethodPost, path: "/api/check", headers: map[string]string{headerRequestedBy: requestedByValue}, status: http.StatusOK},
		{name: "check without the header", method: http.MethodPost, path: "/api/check", status: http.StatusForbidden},
		{name: "check that fails", provider: &mixedStatusProvider{forceErr: errors.New("boom")}, method: http.MethodPost, path: "/api/check", headers: map[string]string{headerRequestedBy: requestedByValue}, status: http.StatusServiceUnavailable},
		{name: "check that times out", provider: &mixedStatusProvider{forceErr: context.DeadlineExceeded}, method: http.MethodPost, path: "/api/check", headers: map[string]string{headerRequestedBy: requestedByValue}, status: http.StatusGatewayTimeout},
		{name: "unknown API path", method: http.MethodGet, path: "/api/nope", status: http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.provider
			if p == nil {
				p = &mixedStatusProvider{}
			}
			rec := do(newServer(t, p), tc.method, tc.path, tc.headers)
			assert.Equal(t, tc.status, rec.Code)
			assert.Equal(t, expected, rec.Header().Get(headerBuild))
		})
	}
}

func TestIntegrationStatusAPIFieldsOfAMixedDomain(t *testing.T) {
	s, err := NewServer(NewServerOpts{HealthChecker: &mixedStatusProvider{}})
	require.NoError(t, err)

	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/status/app.example.com", nil))
	require.Equal(t, http.StatusOK, rec.Code)

	var res struct {
		Endpoints []map[string]any `json:"endpoints"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &res))
	require.Len(t, res.Endpoints, 3)

	assert.Equal(t, map[string]any{
		"healthy":      false,
		"ip":           "192.0.2.10",
		"type":         "A",
		"failureCount": float64(2),
		"since":        "2026-01-02T03:05:05Z",
		"lastFailure":  "2026-01-02T03:05:05Z",
	}, res.Endpoints[0])
	assert.Equal(t, "AAAA", res.Endpoints[1]["type"])
	assert.Equal(t, map[string]any{
		"healthy":    true,
		"ip":         "0a1b2c3d.cfargotunnel.com",
		"type":       "CNAME",
		"proxied":    true,
		"since":      "2026-01-02T03:04:05Z",
		"sinceStart": true,
	}, res.Endpoints[2])
}
