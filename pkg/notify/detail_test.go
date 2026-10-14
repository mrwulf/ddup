package notify

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/italypaleale/ddup/pkg/config"
)

// detailEvent is an event for a domain with two tiers: two primary endpoints and a backup on standby
func detailEvent(typ config.NotificationEvent) Event {
	return Event{
		Type:   typ,
		Domain: "example.com",
		Time:   testTime,
		Endpoints: []EndpointState{
			{Name: "server1", IP: "192.0.2.10", Healthy: true},
			{Name: "server2", IP: "192.0.2.11", Healthy: false},
			{Name: "backup", IP: "192.0.2.20", Healthy: true},
		},
		Published: []string{"192.0.2.10"},
		Previous:  []string{"192.0.2.20"},
		Detail: Detail{
			Endpoints: map[string]EndpointDetail{
				"192.0.2.10": {Priority: 0, Active: true},
				"192.0.2.11": {Priority: 0, Error: "HTTP request failed: dial tcp 192.0.2.11:443: connect: connection refused"},
				"192.0.2.20": {Priority: 1},
			},
		},
	}
}

func detailNotifier(t *testing.T, nc config.ConfigNotifications, deps dependencies) *Notifier {
	t.Helper()
	nc.Detail = true
	return newTestNotifier(t, nc, deps)
}

func TestDetailMessage(t *testing.T) {
	t.Run("dns_updated with tiers", func(t *testing.T) {
		m := withDetail(newMessage(detailEvent(config.EventDNSUpdated)), detailEvent(config.EventDNSUpdated))
		assert.Equal(t, "🟡 example.com now points to 192.0.2.10", m.title)
		assert.Equal(t, "example.com now points to 192.0.2.10", m.ntfyTitle)
		assert.Equal(t, []string{"yellow_circle"}, m.tags)
		assert.Equal(t, `Event: dns_updated
Domain: example.com
Time: 2026-01-02T03:04:05Z
Status: 🟡 warning
Now: 192.0.2.10
Before: 192.0.2.20
Priority: 0 (was 1)

Endpoints:
  server1 (192.0.2.10, priority 0): healthy
  server2 (192.0.2.11, priority 0): UNHEALTHY (HTTP request failed: dial tcp 192.0.2.11:443: connect: connection refused)
  backup (192.0.2.20, priority 1): healthy (standby)
`, m.body)
	})

	t.Run("all_unhealthy with one tier", func(t *testing.T) {
		ev := Event{
			Type:   config.EventAllUnhealthy,
			Domain: "example.com",
			Time:   testTime,
			Endpoints: []EndpointState{
				{Name: "server1", IP: "192.0.2.10"},
				{Name: "server2", IP: "192.0.2.11"},
			},
			Previous: []string{"192.0.2.10", "192.0.2.11"},
			Detail: Detail{
				Endpoints: map[string]EndpointDetail{
					"192.0.2.10": {Error: "status code 503"},
				},
			},
		}
		m := withDetail(newMessage(ev), ev)
		assert.Equal(t, "🔴 No healthy endpoints for example.com", m.title)
		assert.Equal(t, `Event: all_unhealthy
Domain: example.com
Time: 2026-01-02T03:04:05Z
Status: 🔴 unhealthy
Unchanged: 192.0.2.10, 192.0.2.11

Endpoints:
  server1 (192.0.2.10): UNHEALTHY (status code 503)
  server2 (192.0.2.11): UNHEALTHY
`, m.body)
	})

	t.Run("without detail data", func(t *testing.T) {
		ev := testEvent(config.EventDNSUpdated)
		ev.Endpoints[2].Healthy = true
		m := withDetail(newMessage(ev), ev)
		assert.Equal(t, "🟢 example.com now points to 192.0.2.10, 192.0.2.11", m.title)
		assert.Contains(t, m.body, "Status: 🟢 healthy\n")
		assert.NotContains(t, m.body, "Priority:")
		assert.Contains(t, m.body, "  server3 (192.0.2.12): healthy\n")
	})

	t.Run("disabled", func(t *testing.T) {
		n := &Notifier{}
		ev := detailEvent(config.EventDNSUpdated)
		assert.Equal(t, newMessage(ev), n.message(ev))
	})
}

func TestDetailNtfy(t *testing.T) {
	ts := newTestServer(t, nil)

	n := detailNotifier(t, config.ConfigNotifications{
		Ntfy: []config.ConfigNotificationNtfy{
			{
				Name:                       "alerts",
				URL:                        ts.URL + "/ddup",
				ConfigNotificationDelivery: fastDelivery(),
			},
		},
	}, realDeps())

	n.Notify(detailEvent(config.EventDNSUpdateFailed))
	shutdownNotifier(t, n)

	reqs := ts.Requests()
	require.Len(t, reqs, 1)

	// The tag shows the status before the title, so the title has no emoji
	assert.Equal(t, "DNS update failed for example.com", reqs[0].Header.Get("Title"))
	assert.Equal(t, "yellow_circle,warning", reqs[0].Header.Get("Tags"))
	assert.Equal(t, "4", reqs[0].Header.Get("Priority"))
	assert.Contains(t, reqs[0].Body, "Status: 🟡 warning\n")
	assert.Contains(t, reqs[0].Body, "server2 (192.0.2.11, priority 0): UNHEALTHY (HTTP request failed: dial tcp 192.0.2.11:443: connect: connection refused)\n")
	assert.Contains(t, reqs[0].Body, "backup (192.0.2.20, priority 1): healthy (standby)\n")
}

func TestDetailWebhook(t *testing.T) {
	ts := newTestServer(t, nil)

	n := detailNotifier(t, config.ConfigNotifications{
		Webhooks: []config.ConfigNotificationWebhook{
			{
				Name:                       "chat",
				Format:                     "slack",
				URL:                        ts.URL + "/services/T000/B000/" + testHookToken,
				ConfigNotificationDelivery: fastDelivery(),
			},
		},
	}, realDeps())

	n.Notify(detailEvent(config.EventDNSUpdated))
	shutdownNotifier(t, n)

	reqs := ts.Requests()
	require.Len(t, reqs, 1)

	var parsed map[string]string
	err := json.Unmarshal([]byte(reqs[0].Body), &parsed)
	require.NoError(t, err)
	ev := detailEvent(config.EventDNSUpdated)
	assert.Equal(t, "🟡 example.com now points to 192.0.2.10\n\n"+detailBody(ev, ev.status()), parsed["text"])
}

func TestDetailEmail(t *testing.T) {
	fake := &fakeEmailer{}

	n := detailNotifier(t, config.ConfigNotifications{
		Email: []config.ConfigNotificationEmail{
			{
				Name:             "ops",
				ConnectionString: "resend://" + testResendKey + "?fromAddress=ddup@example.com",
				To:               []string{"postmaster@example.com"},
			},
		},
	}, fakeEmailerDeps(fake, nil))

	n.Notify(detailEvent(config.EventAllUnhealthy))
	shutdownNotifier(t, n)

	sent := fake.Sent()
	require.Len(t, sent, 1)
	assert.Equal(t, "🔴 No healthy endpoints for example.com", sent[0].Subject)
	assert.Contains(t, sent[0].Message.Text, "Status: 🔴 unhealthy\n")
	assert.Contains(t, sent[0].Message.Text, "UNHEALTHY (HTTP request failed: dial tcp 192.0.2.11:443: connect: connection refused)\n")
}

func TestDetailErrorsAreRedacted(t *testing.T) {
	ts := newTestServer(t, nil)

	n := detailNotifier(t, config.ConfigNotifications{
		Ntfy: []config.ConfigNotificationNtfy{
			{
				Name:                       "alerts",
				URL:                        ts.URL + "/ddup",
				ConfigNotificationDelivery: fastDelivery(),
			},
		},
	}, realDeps())

	ev := detailEvent(config.EventAllUnhealthy)
	shared := map[string]EndpointDetail{
		"192.0.2.11": {Error: "IP lookup failed: https://lookup.example.com/ip?token=abc123: status code 500\nhttps://other.example.com/: " + testCFToken},
	}
	ev.Detail.Endpoints = shared
	n.Notify(ev)
	shutdownNotifier(t, n)

	reqs := ts.Requests()
	require.Len(t, reqs, 1)
	assert.NotContains(t, reqs[0].Body, testCFToken)
	assert.NotContains(t, reqs[0].Body, "abc123")
	assert.Contains(t, reqs[0].Body, "UNHEALTHY (IP lookup failed: https://lookup.example.com/ip?token=[redacted] status code 500; https://other.example.com/: [redacted])\n")

	// The caller's detail is not changed
	assert.Contains(t, shared["192.0.2.11"].Error, testCFToken)
}

func TestTruncateRunes(t *testing.T) {
	assert.Equal(t, "abc", truncateRunes("abc", 3))
	assert.Equal(t, "ab…", truncateRunes("abc", 2))
	assert.Equal(t, "🟢…", truncateRunes("🟢🟡", 1))
}
