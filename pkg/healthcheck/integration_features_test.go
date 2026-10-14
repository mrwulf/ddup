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
	"github.com/italypaleale/ddup/pkg/notify"
)

// These tests exercise the features of the open pull requests together:
// cname and proxied (typed targets), ipLookupFamily (dynamic DNS), the status times and notifications

// mixedDomain is a domain with a fixed IP and a dynamic endpoint at priority 0, and a proxied CNAME at priority 1
type mixedDomain struct {
	hc       *HealthChecker
	dc       *domainChecker
	mc       *checker.MockChecker
	provider *dns.MockProvider
	notifier *recordingNotifier
	now      time.Time

	fixed   *config.ConfigEndpoint
	dynamic *config.ConfigEndpoint
	tunnel  *config.ConfigEndpoint
}

const mixedDomainName = "app.example.com"

func newMixedDomain() *mixedDomain {
	m := &mixedDomain{
		provider: dns.NewMockProvider(false),
		notifier: &recordingNotifier{},
		mc: &checker.MockChecker{
			Domain:      mixedDomainName,
			MaxAttempts: 1,
		},
		now:     time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC),
		fixed:   &config.ConfigEndpoint{Name: "fixed", IP: "192.0.2.10"},
		dynamic: &config.ConfigEndpoint{Name: "dynamic", IPLookupFamily: 4},
		tunnel: &config.ConfigEndpoint{
			Name:            "tunnel",
			CNAME:           "0a1b2c3d.cfargotunnel.com",
			Priority:        1,
			ProviderOptions: proxiedEndpointOptions(),
		},
	}
	m.dc = &domainChecker{
		checker:    m.mc,
		ttl:        60,
		failedIPs:  make(map[string]int),
		priorities: endpointPriorities([]*config.ConfigEndpoint{m.fixed, m.dynamic, m.tunnel}),
		provider:   m.provider,
		clock:      func() time.Time { return m.now },
	}
	m.hc = &HealthChecker{
		domainCheckers: map[string]*domainChecker{mixedDomainName: m.dc},
		notifier:       m.notifier,
	}
	return m
}

// check runs a cycle where each endpoint is healthy or not, and the dynamic endpoint finds the address
func (m *mixedDomain) check(t *testing.T, fixedOK bool, address string, dynamicOK bool, tunnelOK bool) {
	t.Helper()

	result := func(ep *config.ConfigEndpoint, target string, healthy bool) checker.Result {
		res := checker.Result{Endpoint: ep, Target: target, Healthy: healthy}
		if !healthy {
			res.Error = errors.New("down")
		}
		return res
	}
	m.mc.Results = []checker.Result{
		result(m.fixed, "", fixedOK),
		result(m.dynamic, address, dynamicOK),
		result(m.tunnel, "", tunnelOK),
	}
	m.hc.checkAndUpdateDNS(t.Context())
}

var (
	fixedTarget  = dns.Target{Type: "A", Value: "192.0.2.10"}
	tunnelTarget = dns.Target{Type: "CNAME", Value: "0a1b2c3d.cfargotunnel.com", Proxied: true}
)

func dynamicTarget(address string) dns.Target {
	return dns.Target{Type: "A", Value: address}
}

func TestIntegrationMixedDomainPublishesAndNotifies(t *testing.T) {
	m := newMixedDomain()
	m.provider.Previous = []string{"198.51.100.1"}

	// All healthy: the IP tier is published, with the address that was looked up, and the CNAME is on standby
	m.check(t, true, "203.0.113.5", true, true)
	assert.ElementsMatch(t, []dns.Target{fixedTarget, dynamicTarget("203.0.113.5")}, m.provider.LastTargets)
	require.Len(t, m.notifier.Events(), 1)
	ev := m.notifier.Events()[0]
	assert.Equal(t, config.EventDNSUpdated, ev.Type)
	assert.ElementsMatch(t, []string{"192.0.2.10", "203.0.113.5"}, ev.Published)
	assert.Equal(t, []string{"198.51.100.1"}, ev.Previous)
	assert.ElementsMatch(t, []notify.EndpointState{
		{Name: "fixed", IP: "192.0.2.10", Healthy: true},
		{Name: "dynamic", IP: "203.0.113.5", Healthy: true},
		{Name: "tunnel", IP: "0a1b2c3d.cfargotunnel.com", Healthy: true},
	}, ev.Endpoints)

	// Nothing changes: no update and no notification
	m.check(t, true, "203.0.113.5", true, true)
	assert.Equal(t, 1, m.provider.CallCount)
	assert.Len(t, m.notifier.Events(), 1)

	// The dynamic address changes: DNS follows, and the notification has the new address
	m.provider.Previous = []string{"192.0.2.10", "203.0.113.5"}
	m.check(t, true, "203.0.113.6", true, true)
	assert.Equal(t, 2, m.provider.CallCount)
	assert.ElementsMatch(t, []dns.Target{fixedTarget, dynamicTarget("203.0.113.6")}, m.provider.LastTargets)
	require.Len(t, m.notifier.Events(), 2)
	ev = m.notifier.Events()[1]
	assert.Equal(t, config.EventDNSUpdated, ev.Type)
	assert.ElementsMatch(t, []string{"192.0.2.10", "203.0.113.6"}, ev.Published)
	assert.ElementsMatch(t, []string{"192.0.2.10", "203.0.113.5"}, ev.Previous)

	// The whole IP tier fails: the CNAME tier is published, and it's proxied
	m.provider.Previous = []string{"192.0.2.10", "203.0.113.6"}
	m.check(t, false, "203.0.113.6", false, true)
	assert.Equal(t, 3, m.provider.CallCount)
	assert.Equal(t, []dns.Target{tunnelTarget}, m.provider.LastTargets)
	events := m.notifier.Events()
	require.Len(t, events, 3, "a domain with a healthy CNAME is not an outage")
	ev = events[2]
	assert.Equal(t, config.EventDNSUpdated, ev.Type)
	assert.Equal(t, []string{"0a1b2c3d.cfargotunnel.com"}, ev.Published)
	assert.ElementsMatch(t, []notify.EndpointState{
		{Name: "fixed", IP: "192.0.2.10", Healthy: false},
		{Name: "dynamic", IP: "203.0.113.6", Healthy: false},
		{Name: "tunnel", IP: "0a1b2c3d.cfargotunnel.com", Healthy: true},
	}, ev.Endpoints)

	// The CNAME fails too: all unhealthy, DNS is left alone
	m.check(t, false, "203.0.113.6", false, false)
	assert.Equal(t, 3, m.provider.CallCount)
	events = m.notifier.Events()
	require.Len(t, events, 4)
	assert.Equal(t, config.EventAllUnhealthy, events[3].Type)
	assert.Equal(t, []string{"0a1b2c3d.cfargotunnel.com"}, events[3].Previous, "the previous addresses are the CNAME that is still published")

	// The dynamic endpoint comes back with yet another address: back to the IP tier
	m.provider.Previous = []string{"0a1b2c3d.cfargotunnel.com"}
	m.check(t, false, "203.0.113.7", true, false)
	assert.Equal(t, []dns.Target{dynamicTarget("203.0.113.7")}, m.provider.LastTargets)
	events = m.notifier.Events()
	require.Len(t, events, 5)
	assert.Equal(t, config.EventDNSUpdated, events[4].Type)
	assert.Equal(t, []string{"203.0.113.7"}, events[4].Published)
	assert.Equal(t, []string{"0a1b2c3d.cfargotunnel.com"}, events[4].Previous)

	// Priorities of old dynamic addresses are forgotten
	assert.NotContains(t, m.dc.getPriorities(), "203.0.113.5")
	assert.NotContains(t, m.dc.getPriorities(), "203.0.113.6")
}

// The status of a mixed domain: type, proxied and the times of each kind of endpoint
func TestIntegrationMixedDomainStatus(t *testing.T) {
	m := newMixedDomain()
	start := m.now

	m.check(t, true, "203.0.113.5", true, true)
	status := m.hc.GetDomainStatus(mixedDomainName)
	require.NotNil(t, status)
	byIP := statusByIP(status.Endpoints)
	require.Len(t, byIP, 3)

	assert.Equal(t, "A", byIP["192.0.2.10"].Type)
	assert.Equal(t, "A", byIP["203.0.113.5"].Type)
	assert.Equal(t, "CNAME", byIP["0a1b2c3d.cfargotunnel.com"].Type)
	assert.False(t, byIP["192.0.2.10"].Proxied)
	assert.False(t, byIP["203.0.113.5"].Proxied)
	assert.True(t, byIP["0a1b2c3d.cfargotunnel.com"].Proxied)
	for ip, e := range byIP {
		require.NotNil(t, e.Since, ip)
		assert.True(t, start.Equal(*e.Since), ip)
		assert.True(t, e.SinceStart, "%s: the time is when ddup first saw the endpoint", ip)
		assert.Nil(t, e.LastFailure, ip)
	}

	// Later, the IP tier fails
	failedAt := start.Add(5 * time.Minute)
	m.now = failedAt
	m.check(t, false, "203.0.113.5", false, true)

	status = m.hc.GetDomainStatus(mixedDomainName)
	byIP = statusByIP(status.Endpoints)
	require.Len(t, byIP, 3)
	for _, ip := range []string{"192.0.2.10", "203.0.113.5"} {
		e := byIP[ip]
		assert.False(t, e.Healthy, ip)
		require.NotNil(t, e.Since, ip)
		assert.True(t, failedAt.Equal(*e.Since), ip)
		assert.False(t, e.SinceStart, ip)
		require.NotNil(t, e.LastFailure, ip)
		assert.True(t, failedAt.Equal(*e.LastFailure), ip)
	}
	tunnel := byIP["0a1b2c3d.cfargotunnel.com"]
	assert.True(t, tunnel.Healthy)
	require.NotNil(t, tunnel.Since)
	assert.True(t, start.Equal(*tunnel.Since), "the CNAME endpoint kept its time, since it didn't change state")
	assert.True(t, tunnel.SinceStart)
	assert.Nil(t, tunnel.LastFailure)
	assert.Equal(t, "CNAME", tunnel.Type)
	assert.True(t, tunnel.Proxied)

	// The JSON has every field, with the names the dashboard reads
	enc, err := json.Marshal(status)
	require.NoError(t, err)
	var decoded struct {
		Endpoints []map[string]any `json:"endpoints"`
	}
	require.NoError(t, json.Unmarshal(enc, &decoded))
	require.Len(t, decoded.Endpoints, 3)
	for _, e := range decoded.Endpoints {
		assert.Contains(t, e, "ip")
		assert.Contains(t, e, "healthy")
		assert.Contains(t, e, "type")
		assert.Contains(t, e, "since")
		if e["type"] == "CNAME" {
			assert.Equal(t, true, e["sinceStart"])
		} else {
			assert.NotContains(t, e, "sinceStart", "it is omitted when false")
		}
		if e["healthy"] == false {
			assert.Contains(t, e, "lastFailure")
			assert.Contains(t, e, "failureCount")
		}
		if e["type"] == "CNAME" {
			assert.Equal(t, true, e["proxied"])
		}
	}
}

// The address of a dynamic endpoint changes: the new address starts its own history, and the old one is forgotten
func TestIntegrationDynamicAddressChangeStatus(t *testing.T) {
	m := newMixedDomain()
	m.check(t, true, "203.0.113.5", true, true)

	changedAt := m.now.Add(time.Hour)
	m.now = changedAt
	m.check(t, true, "203.0.113.6", true, true)

	byIP := statusByIP(m.hc.GetDomainStatus(mixedDomainName).Endpoints)
	require.Len(t, byIP, 3)
	assert.NotContains(t, byIP, "203.0.113.5")
	e := byIP["203.0.113.6"]
	require.NotNil(t, e.Since)
	assert.True(t, changedAt.Equal(*e.Since))
}

func statusByIP(endpoints []DomainStatusEndpoint) map[string]DomainStatusEndpoint {
	res := make(map[string]DomainStatusEndpoint, len(endpoints))
	for _, e := range endpoints {
		res[e.IP] = e
	}
	return res
}
