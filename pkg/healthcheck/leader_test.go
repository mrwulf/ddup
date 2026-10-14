package healthcheck

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/italypaleale/ddup/pkg/config"
)

type fakeElector struct{ leader atomic.Bool }

func (*fakeElector) Run(ctx context.Context) error { <-ctx.Done(); return nil }
func (f *fakeElector) IsLeader() bool              { return f.leader.Load() }
func (*fakeElector) Leader() string                { return "" }

func newElector(leader bool) *fakeElector {
	el := &fakeElector{}
	el.leader.Store(leader)
	return el
}

func TestHealthChecker_OnlyLeaderUpdatesDNS(t *testing.T) {
	hc, mc, provider, endpoints := newNotifyTestChecker(nil)
	el := newElector(false)
	hc.SetElector(el)
	setHealth(mc, endpoints, true, true)

	// Standby: tracks health but never touches DNS
	hc.checkAndUpdateDNS(t.Context())
	assert.Equal(t, 0, provider.CallCount)
	assert.ElementsMatch(t, []string{"192.0.2.10", "192.0.2.11"}, hc.domainCheckers["example.com"].healthyIPs, "standby keeps state warm")

	// Takeover: publishes even though the healthy IPs didn't change since the last cycle
	el.leader.Store(true)
	hc.checkAndUpdateDNS(t.Context())
	assert.Equal(t, 1, provider.CallCount)

	// Steady state as leader: nothing changed, no more calls
	hc.checkAndUpdateDNS(t.Context())
	assert.Equal(t, 1, provider.CallCount)

	// Losing leadership and getting it back is another takeover
	el.leader.Store(false)
	hc.checkAndUpdateDNS(t.Context())
	assert.Equal(t, 1, provider.CallCount)
	el.leader.Store(true)
	hc.checkAndUpdateDNS(t.Context())
	assert.Equal(t, 2, provider.CallCount)
}

func TestHealthChecker_StandbyKeepsItsOwnTimes(t *testing.T) {
	hc, mc, _, endpoints := newNotifyTestChecker(nil)
	hc.SetElector(newElector(false))
	setHealth(mc, endpoints, true, false)

	hc.checkAndUpdateDNS(t.Context())

	status := hc.GetDomainStatus("example.com")
	require.NotNil(t, status)
	require.Len(t, status.Endpoints, 2)
	for _, e := range status.Endpoints {
		require.NotNil(t, e.Since, "endpoint %s", e.IP)
		assert.True(t, e.SinceStart, "a replica only knows since when it checked")
	}
}

func TestHealthChecker_StandbySendsNoNotifications(t *testing.T) {
	rec := &recordingNotifier{}
	hc, mc, provider, endpoints := newNotifyTestChecker(rec)
	hc.SetElector(newElector(false))

	// DNS change, outage, then a failing provider: a standby does none of it
	setHealth(mc, endpoints, true, true)
	hc.checkAndUpdateDNS(t.Context())
	setHealth(mc, endpoints, false, false)
	hc.checkAndUpdateDNS(t.Context())
	provider.ShouldError = true
	setHealth(mc, endpoints, true, false)
	hc.checkAndUpdateDNS(t.Context())

	assert.Empty(t, rec.Events())
	assert.Equal(t, 0, provider.CallCount)
}

func TestHealthChecker_TakeoverResyncs(t *testing.T) {
	t.Run("sends dns_updated if the old leader had not finished", func(t *testing.T) {
		rec := &recordingNotifier{}
		hc, mc, provider, endpoints := newNotifyTestChecker(rec)
		el := newElector(false)
		hc.SetElector(el)
		setHealth(mc, endpoints, true, true)
		hc.checkAndUpdateDNS(t.Context())

		el.leader.Store(true)
		hc.checkAndUpdateDNS(t.Context())
		assert.Equal(t, 1, provider.CallCount)
		assert.Equal(t, []config.NotificationEvent{config.EventDNSUpdated}, rec.Types())
	})

	t.Run("sends nothing if the records are already right", func(t *testing.T) {
		rec := &recordingNotifier{}
		hc, mc, provider, endpoints := newNotifyTestChecker(rec)
		provider.NoChange = true
		el := newElector(false)
		hc.SetElector(el)
		setHealth(mc, endpoints, true, true)
		hc.checkAndUpdateDNS(t.Context())

		el.leader.Store(true)
		hc.checkAndUpdateDNS(t.Context())
		assert.Equal(t, 1, provider.CallCount, "DNS is re-asserted")
		assert.Empty(t, rec.Events(), "and nobody is told when nothing changed")
	})

	t.Run("the failure of the new leader is reported", func(t *testing.T) {
		rec := &recordingNotifier{}
		hc, mc, provider, endpoints := newNotifyTestChecker(rec)
		provider.ShouldError = true
		el := newElector(false)
		hc.SetElector(el)
		setHealth(mc, endpoints, true, true)
		hc.checkAndUpdateDNS(t.Context())

		el.leader.Store(true)
		hc.checkAndUpdateDNS(t.Context())
		hc.checkAndUpdateDNS(t.Context())
		require.Equal(t, []config.NotificationEvent{config.EventDNSUpdateFailed}, rec.Types(), "reported once, and retried at every cycle without repeating it")
		assert.NotEmpty(t, rec.Events()[0].Error)
		assert.Equal(t, 2, provider.CallCount, "the failed update is retried on the next cycle")
	})
}

// Two replicas of the same deployment share the notification destinations and the DNS provider
func TestHealthChecker_TwoReplicasAnnounceAnOutageOnce(t *testing.T) {
	rec := &recordingNotifier{}
	a, mcA, provider, endpoints := newNotifyTestChecker(rec)
	b, mcB, _, _ := newNotifyTestChecker(rec)
	b.domainCheckers["example.com"].provider = provider
	elA, elB := newElector(true), newElector(false)
	a.SetElector(elA)
	b.SetElector(elB)

	cycle := func(healthy ...bool) {
		setHealth(mcA, endpoints, healthy...)
		setHealth(mcB, endpoints, healthy...)
		a.checkAndUpdateDNS(t.Context())
		b.checkAndUpdateDNS(t.Context())
	}

	cycle(true, true)
	require.Equal(t, []config.NotificationEvent{config.EventDNSUpdated}, rec.Types())

	// The outage starts: only the leader announces it
	cycle(false, false)
	require.Equal(t, []config.NotificationEvent{config.EventDNSUpdated, config.EventAllUnhealthy}, rec.Types())

	// The leader changes during the outage: the new leader does not announce it again
	elA.leader.Store(false)
	elB.leader.Store(true)
	cycle(false, false)
	cycle(false, false)
	assert.Equal(t, []config.NotificationEvent{config.EventDNSUpdated, config.EventAllUnhealthy}, rec.Types())

	// The recovery is reported once, by the new leader
	cycle(true, false)
	assert.Equal(t, []config.NotificationEvent{config.EventDNSUpdated, config.EventAllUnhealthy, config.EventDNSUpdated}, rec.Types())
	assert.Equal(t, []string{"192.0.2.10"}, rec.Events()[2].Published)

	// A new outage is announced once, by the new leader
	cycle(false, false)
	cycle(false, false)
	assert.Equal(t, []config.NotificationEvent{config.EventDNSUpdated, config.EventAllUnhealthy, config.EventDNSUpdated, config.EventAllUnhealthy}, rec.Types())
}

func TestHealthChecker_NoElectorActs(t *testing.T) {
	rec := &recordingNotifier{}
	hc, mc, provider, endpoints := newNotifyTestChecker(rec)
	setHealth(mc, endpoints, true, true)
	hc.checkAndUpdateDNS(t.Context())
	assert.Equal(t, 1, provider.CallCount)
	assert.Equal(t, []config.NotificationEvent{config.EventDNSUpdated}, rec.Types())
}
