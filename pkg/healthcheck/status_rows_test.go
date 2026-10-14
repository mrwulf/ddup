package healthcheck

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/italypaleale/ddup/pkg/config"
	"github.com/italypaleale/ddup/pkg/dns"
	"github.com/italypaleale/ddup/pkg/healthcheck/checker"
)

func TestStatus_NamePriorityAndPublishedTier(t *testing.T) {
	endpoints := []*config.ConfigEndpoint{
		{Name: "vps", URL: "https://vps.example.com/health", IP: "192.0.2.10"},
		// The default name is the URL, which is not worth showing
		{Name: "https://vps2.example.com/health", URL: "https://vps2.example.com/health", IP: "192.0.2.11"},
		{Name: "tunnel", URL: "https://tunnel.example.com/health", CNAME: "0a1b2c3d.cfargotunnel.com", Priority: 1, ProviderOptions: proxiedEndpointOptions()},
	}
	mc := &checker.MockChecker{Domain: "example.com", MaxAttempts: 1}
	hc := &HealthChecker{
		domainCheckers: map[string]*domainChecker{
			"example.com": {
				checker:    mc,
				ttl:        60,
				priorities: endpointPriorities(endpoints),
				names:      endpointNames(endpoints),
				failedIPs:  make(map[string]int),
				provider:   dns.NewMockProvider(false),
			},
		},
	}
	setHealth := func(healthy ...bool) []DomainStatusEndpoint {
		mc.Results = make([]checker.Result, len(endpoints))
		for i, ep := range endpoints {
			mc.Results[i] = checker.Result{Endpoint: ep, Healthy: healthy[i]}
			if !healthy[i] {
				mc.Results[i].Error = errors.New("down")
			}
		}
		hc.checkAndUpdateDNS(t.Context())
		status := hc.GetDomainStatus("example.com")
		require.NotNil(t, status)
		return withoutTimes(status.Endpoints)
	}

	// The first tier is published, the tunnel is healthy on standby
	assert.ElementsMatch(t, []DomainStatusEndpoint{
		{Name: "vps", Healthy: true, Active: true, IP: "192.0.2.10", Type: "A"},
		{Healthy: true, Active: true, IP: "192.0.2.11", Type: "A"},
		{Name: "tunnel", Healthy: true, IP: "0a1b2c3d.cfargotunnel.com", Type: "CNAME", Proxied: true, Priority: 1},
	}, setHealth(true, true, true))

	// Both A records are down: the tunnel is published
	assert.ElementsMatch(t, []DomainStatusEndpoint{
		{Name: "vps", Healthy: false, IP: "192.0.2.10", Type: "A", FailureCount: 1},
		{Healthy: false, IP: "192.0.2.11", Type: "A", FailureCount: 1},
		{Name: "tunnel", Healthy: true, Active: true, IP: "0a1b2c3d.cfargotunnel.com", Type: "CNAME", Proxied: true, Priority: 1},
	}, setHealth(false, false, true))
}

func TestStatus_DynamicEndpointName(t *testing.T) {
	home := &config.ConfigEndpoint{Name: "home", IPLookupFamily: 4, Priority: 2}
	mc := &checker.MockChecker{Domain: "home.example.com", MaxAttempts: 2}
	hc := &HealthChecker{
		domainCheckers: map[string]*domainChecker{
			"home.example.com": {
				checker:    mc,
				ttl:        60,
				priorities: endpointPriorities([]*config.ConfigEndpoint{home}),
				names:      endpointNames([]*config.ConfigEndpoint{home}),
				failedIPs:  make(map[string]int),
				provider:   dns.NewMockProvider(false),
			},
		},
	}
	run := func(ip string) []DomainStatusEndpoint {
		mc.Results = []checker.Result{{Endpoint: home, Target: ip, Healthy: true}}
		hc.checkAndUpdateDNS(t.Context())
		return withoutTimes(hc.GetDomainStatus("home.example.com").Endpoints)
	}

	assert.Equal(t, []DomainStatusEndpoint{{Name: "home", Healthy: true, Active: true, IP: "203.0.113.1", Type: "A", Priority: 2}}, run("203.0.113.1"))
	assert.Equal(t, []DomainStatusEndpoint{{Name: "home", Healthy: true, Active: true, IP: "203.0.113.2", Type: "A", Priority: 2}}, run("203.0.113.2"))
	run("203.0.113.3")
	run("203.0.113.4")
	// Only the current address and the one before it are kept
	assert.Len(t, hc.domainCheckers["home.example.com"].dynamicNames, 2, "the names of old addresses are forgotten")
}
