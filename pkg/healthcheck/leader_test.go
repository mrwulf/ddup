//go:build unit

package healthcheck

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/italypaleale/ddup/pkg/config"
	"github.com/italypaleale/ddup/pkg/dns"
	"github.com/italypaleale/ddup/pkg/healthcheck/checker"
)

type fakeElector struct{ leader atomic.Bool }

func (*fakeElector) Run(ctx context.Context) error { <-ctx.Done(); return nil }
func (f *fakeElector) IsLeader() bool              { return f.leader.Load() }

func TestHealthChecker_OnlyLeaderUpdatesDNS(t *testing.T) {
	provider := dns.NewMockProvider(false)
	ep := &config.ConfigEndpoint{Name: "e1", IP: "1.1.1.1"}
	el := &fakeElector{}
	hc := &HealthChecker{
		elector: el,
		domainCheckers: map[string]*domainChecker{
			"example.com": {
				checker:   &checker.MockChecker{Domain: "example.com", MaxAttempts: 2, Results: []checker.Result{{Endpoint: ep, Healthy: true}}},
				ttl:       60,
				failedIPs: map[string]int{},
				provider:  provider,
			},
		},
	}

	// Standby: tracks health but never touches DNS
	hc.checkAndUpdateDNS(t.Context())
	assert.Equal(t, 0, provider.CallCount)
	assert.Equal(t, []string{"1.1.1.1"}, hc.domainCheckers["example.com"].healthyIPs, "standby keeps state warm")

	// Takeover: publishes even though healthy IPs didn't change since the last cycle
	el.leader.Store(true)
	hc.checkAndUpdateDNS(t.Context())
	assert.Equal(t, 1, provider.CallCount)

	// Steady state as leader: nothing changed, no more calls
	hc.checkAndUpdateDNS(t.Context())
	assert.Equal(t, 1, provider.CallCount)
}
