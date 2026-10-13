package healthcheck

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/italypaleale/ddup/pkg/config"
	"github.com/italypaleale/ddup/pkg/dns"
	"github.com/italypaleale/ddup/pkg/healthcheck/checker"
)

// Builds the expected status of an IPv4 endpoint without a name, a priority or timestamps
// It's published (active) if healthy
func plainEndpoint(ip string, healthy bool, failureCount int) DomainStatusEndpoint {
	return DomainStatusEndpoint{Healthy: healthy, IP: ip, Type: "A", Active: healthy, FailureCount: failureCount}
}

// Removes the timestamps from the endpoints of a status, which are covered by since_test.go
func withoutTimes(endpoints []DomainStatusEndpoint) []DomainStatusEndpoint {
	res := make([]DomainStatusEndpoint, len(endpoints))
	for i, e := range endpoints {
		e.Since = nil
		e.SinceStart = false
		e.LastFailure = nil
		res[i] = e
	}
	return res
}

func TestGetDomainStatus(t *testing.T) {
	tests := []struct {
		name       string
		healthyIPs []string
		failedIPs  map[string]int
		expected   []DomainStatusEndpoint
	}{
		{
			name:       "all healthy",
			healthyIPs: []string{"1.1.1.1", "2.2.2.2"},
			failedIPs:  map[string]int{},
			expected: []DomainStatusEndpoint{
				plainEndpoint("1.1.1.1", true, 0),
				plainEndpoint("2.2.2.2", true, 0),
			},
		},
		{
			name:       "endpoint being retried is still healthy",
			healthyIPs: []string{"1.1.1.1", "2.2.2.2"},
			failedIPs:  map[string]int{"2.2.2.2": 1},
			expected: []DomainStatusEndpoint{
				plainEndpoint("1.1.1.1", true, 0),
				plainEndpoint("2.2.2.2", true, 1),
			},
		},
		{
			name:       "endpoint that reached max attempts is unhealthy",
			healthyIPs: []string{"1.1.1.1"},
			failedIPs:  map[string]int{"2.2.2.2": 2},
			expected: []DomainStatusEndpoint{
				plainEndpoint("1.1.1.1", true, 0),
				plainEndpoint("2.2.2.2", false, 2),
			},
		},
		{
			// Before the fix, this endpoint was missing from the status until it reached max attempts
			name:       "endpoint that was never healthy is unhealthy before max attempts",
			healthyIPs: []string{"1.1.1.1"},
			failedIPs:  map[string]int{"2.2.2.2": 1},
			expected: []DomainStatusEndpoint{
				plainEndpoint("1.1.1.1", true, 0),
				plainEndpoint("2.2.2.2", false, 1),
			},
		},
		{
			name:       "no healthy endpoints",
			healthyIPs: nil,
			failedIPs:  map[string]int{"1.1.1.1": 5, "2.2.2.2": 1},
			expected: []DomainStatusEndpoint{
				plainEndpoint("1.1.1.1", false, 5),
				plainEndpoint("2.2.2.2", false, 1),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hc := &HealthChecker{
				domainCheckers: map[string]*domainChecker{
					"example.com": {
						checker:    &checker.MockChecker{Domain: "example.com", MaxAttempts: 2},
						healthyIPs: tc.healthyIPs,
						failedIPs:  tc.failedIPs,
						provider:   dns.NewMockProvider(false),
					},
				},
			}

			status := hc.GetDomainStatus("example.com")
			require.NotNil(t, status)
			assert.ElementsMatch(t, tc.expected, status.Endpoints)
			assert.Equal(t, "mock", status.Provider)
		})
	}

	t.Run("unknown domain", func(t *testing.T) {
		hc := &HealthChecker{domainCheckers: map[string]*domainChecker{}}
		assert.Nil(t, hc.GetDomainStatus("missing.example.com"))
	})

	t.Run("domain not checked yet", func(t *testing.T) {
		hc := &HealthChecker{
			domainCheckers: map[string]*domainChecker{
				"example.com": {checker: &checker.MockChecker{MaxAttempts: 2}, provider: dns.NewMockProvider(false)},
			},
		}

		// The dashboard reads the length of the endpoints array, so it must not be null
		enc, err := json.Marshal(hc.GetDomainStatus("example.com"))
		require.NoError(t, err)
		assert.Contains(t, string(enc), `"endpoints":[]`)
	})
}

func TestGetDomainStatus_AfterChecks(t *testing.T) {
	ep1 := &config.ConfigEndpoint{Name: "endpoint1", IP: "1.1.1.1"}
	ep2 := &config.ConfigEndpoint{Name: "endpoint2", IP: "2.2.2.2"}
	healthy := func(ep *config.ConfigEndpoint) checker.Result {
		return checker.Result{Endpoint: ep, Healthy: true}
	}
	unhealthy := func(ep *config.ConfigEndpoint) checker.Result {
		return checker.Result{Endpoint: ep, Healthy: false, Error: errors.New("down")}
	}

	bothEndpoints := map[string]*config.ConfigEndpoint{"1.1.1.1": ep1, "2.2.2.2": ep2}
	named := func(name, ip string, healthy bool, failureCount int) DomainStatusEndpoint {
		e := plainEndpoint(ip, healthy, failureCount)
		e.Name = name
		return e
	}

	newHC := func(mc *checker.MockChecker, endpoints map[string]*config.ConfigEndpoint) *HealthChecker {
		return &HealthChecker{
			domainCheckers: map[string]*domainChecker{
				"example.com": {
					checker:   mc,
					ttl:       60,
					failedIPs: make(map[string]int),
					endpoints: endpoints,
					provider:  dns.NewMockProvider(false),
				},
			},
		}
	}

	t.Run("endpoint failing at startup is listed", func(t *testing.T) {
		mc := &checker.MockChecker{Domain: "example.com", MaxAttempts: 3, Results: []checker.Result{healthy(ep1), unhealthy(ep2)}}
		hc := newHC(mc, bothEndpoints)

		hc.checkAndUpdateDNS(t.Context())
		assert.ElementsMatch(t, []DomainStatusEndpoint{
			named("endpoint1", "1.1.1.1", true, 0),
			named("endpoint2", "2.2.2.2", false, 1),
		}, withoutTimes(hc.GetDomainStatus("example.com").Endpoints))

		hc.checkAndUpdateDNS(t.Context())
		assert.ElementsMatch(t, []DomainStatusEndpoint{
			named("endpoint1", "1.1.1.1", true, 0),
			named("endpoint2", "2.2.2.2", false, 2),
		}, withoutTimes(hc.GetDomainStatus("example.com").Endpoints))
	})

	t.Run("recovering endpoint stays unhealthy until it is added back", func(t *testing.T) {
		mc := &checker.MockChecker{Domain: "example.com", MaxAttempts: 1, RecoverAfter: 2, Results: []checker.Result{healthy(ep1), healthy(ep2)}}
		hc := newHC(mc, bothEndpoints)
		hc.checkAndUpdateDNS(t.Context())

		// The endpoint fails and is removed
		mc.Results = []checker.Result{healthy(ep1), unhealthy(ep2)}
		hc.checkAndUpdateDNS(t.Context())
		assert.ElementsMatch(t, []DomainStatusEndpoint{
			named("endpoint1", "1.1.1.1", true, 0),
			named("endpoint2", "2.2.2.2", false, 1),
		}, withoutTimes(hc.GetDomainStatus("example.com").Endpoints))

		// First successful check: still recovering
		mc.Results = []checker.Result{healthy(ep1), healthy(ep2)}
		hc.checkAndUpdateDNS(t.Context())
		assert.ElementsMatch(t, []DomainStatusEndpoint{
			named("endpoint1", "1.1.1.1", true, 0),
			named("endpoint2", "2.2.2.2", false, 1),
		}, withoutTimes(hc.GetDomainStatus("example.com").Endpoints))

		// Second successful check: added back
		hc.checkAndUpdateDNS(t.Context())
		assert.ElementsMatch(t, []DomainStatusEndpoint{
			named("endpoint1", "1.1.1.1", true, 0),
			named("endpoint2", "2.2.2.2", true, 0),
		}, withoutTimes(hc.GetDomainStatus("example.com").Endpoints))
	})

	t.Run("standby endpoint is healthy even if not published", func(t *testing.T) {
		standby := &config.ConfigEndpoint{Name: "endpoint2", IP: "2.2.2.2", Priority: 1}
		mc := &checker.MockChecker{Domain: "example.com", MaxAttempts: 2, Results: []checker.Result{healthy(ep1), healthy(&config.ConfigEndpoint{Name: "endpoint2", IP: "2.2.2.2", Priority: 1})}}
		hc := newHC(mc, map[string]*config.ConfigEndpoint{"1.1.1.1": ep1, "2.2.2.2": standby})
		mockProvider := dns.NewMockProvider(false)
		hc.domainCheckers["example.com"].provider = mockProvider

		hc.checkAndUpdateDNS(t.Context())
		assert.Equal(t, []string{"1.1.1.1"}, mockProvider.LastIPs)
		// The standby endpoint is healthy but not published
		expectedStandby := named("endpoint2", "2.2.2.2", true, 0)
		expectedStandby.Priority = 1
		expectedStandby.Active = false
		assert.ElementsMatch(t, []DomainStatusEndpoint{
			named("endpoint1", "1.1.1.1", true, 0),
			expectedStandby,
		}, withoutTimes(hc.GetDomainStatus("example.com").Endpoints))
	})

	t.Run("DNS error is reported", func(t *testing.T) {
		mc := &checker.MockChecker{Domain: "example.com", MaxAttempts: 2, Results: []checker.Result{healthy(ep1)}}
		hc := newHC(mc, bothEndpoints)
		hc.domainCheckers["example.com"].provider = dns.NewMockProvider(true)

		before := time.Now()
		hc.checkAndUpdateDNS(t.Context())
		status := hc.GetDomainStatus("example.com")
		assert.Contains(t, status.Error, "mock error")
		assert.False(t, status.LastUpdated.Before(before))
	})
}

func TestGetAllDomainsStatus(t *testing.T) {
	hc := &HealthChecker{
		domainCheckers: map[string]*domainChecker{
			"a.example.com": {
				checker:    &checker.MockChecker{MaxAttempts: 2},
				healthyIPs: []string{"1.1.1.1"},
				provider:   dns.NewMockProvider(false),
			},
			"b.example.com": {
				checker:   &checker.MockChecker{MaxAttempts: 2},
				failedIPs: map[string]int{"2.2.2.2": 3},
				lastError: "Error updating DNS records: boom",
				provider:  dns.NewMockProvider(false),
			},
		},
	}

	all := hc.GetAllDomainsStatus()
	require.Len(t, all, 2)
	assert.Equal(t, []DomainStatusEndpoint{plainEndpoint("1.1.1.1", true, 0)}, all["a.example.com"].Endpoints)
	assert.Empty(t, all["a.example.com"].Error)
	assert.Equal(t, []DomainStatusEndpoint{plainEndpoint("2.2.2.2", false, 3)}, all["b.example.com"].Endpoints)
	assert.Equal(t, "Error updating DNS records: boom", all["b.example.com"].Error)
}

// The dashboard depends on these field names: see the DomainStatus interface in dashboard/src/components/DomainMonitorDashboard.tsx
func TestDomainStatusJSON(t *testing.T) {
	since := time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)
	lastFailure := time.Date(2026, 1, 2, 2, 0, 0, 0, time.UTC)
	status := DomainStatus{
		LastUpdated: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Provider:    "cf",
		Error:       "boom",
		Endpoints: []DomainStatusEndpoint{
			{Healthy: true, IP: "192.0.2.1", Type: "A", Active: true},
			{Healthy: false, IP: "2001:db8::1", Type: "AAAA", FailureCount: 3},
			{Name: "tunnel", Healthy: true, IP: "tunnel.example.com", Type: "CNAME", Proxied: true, Priority: 2, Since: &since, SinceStart: true, LastFailure: &lastFailure},
		},
	}
	enc, err := json.Marshal(status)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"lastUpdated": "2026-01-02T03:04:05Z",
		"provider": "cf",
		"error": "boom",
		"endpoints": [
			{"healthy": true, "ip": "192.0.2.1", "type": "A", "priority": 0, "active": true},
			{"healthy": false, "ip": "2001:db8::1", "type": "AAAA", "priority": 0, "active": false, "failureCount": 3},
			{"name": "tunnel", "healthy": true, "ip": "tunnel.example.com", "type": "CNAME", "proxied": true, "priority": 2, "active": false, "since": "2026-01-02T03:00:00Z", "sinceStart": true, "lastFailure": "2026-01-02T02:00:00Z"}
		]
	}`, string(enc))

	// Optional fields are omitted when empty
	enc, err = json.Marshal(DomainStatus{Provider: "cf", Endpoints: []DomainStatusEndpoint{}})
	require.NoError(t, err)
	assert.JSONEq(t, `{"lastUpdated": "0001-01-01T00:00:00Z", "provider": "cf", "endpoints": []}`, string(enc))
}
