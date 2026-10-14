package notify

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/italypaleale/ddup/pkg/config"
)

// The notifications for the endpoints that the health checker publishes under their target: a CNAME hostname and a looked-up address
func TestIntegrationMessageForCNAMEAndLookedUpAddress(t *testing.T) {
	endpoints := []EndpointState{
		{Name: "fixed", IP: "192.0.2.10", Healthy: false},
		{Name: "dynamic", IP: "203.0.113.6", Healthy: false},
		{Name: "tunnel", IP: "0a1b2c3d.cfargotunnel.com", Healthy: true},
	}

	t.Run("switch to the CNAME tier", func(t *testing.T) {
		m := newMessage(Event{
			Type:      config.EventDNSUpdated,
			Domain:    "app.example.com",
			Time:      time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
			Endpoints: endpoints,
			Published: []string{"0a1b2c3d.cfargotunnel.com"},
			Previous:  []string{"203.0.113.6", "192.0.2.10"},
		})
		assert.Equal(t, "app.example.com now points to 0a1b2c3d.cfargotunnel.com", m.title)
		assert.Contains(t, m.body, "Now: 0a1b2c3d.cfargotunnel.com\n")
		assert.Contains(t, m.body, "Before: 192.0.2.10, 203.0.113.6\n")
		assert.Contains(t, m.body, "  tunnel (0a1b2c3d.cfargotunnel.com): healthy\n")
		assert.Contains(t, m.body, "  dynamic (203.0.113.6): UNHEALTHY\n")
	})

	t.Run("looked-up address changes", func(t *testing.T) {
		m := newMessage(Event{
			Type:      config.EventDNSUpdated,
			Domain:    "app.example.com",
			Endpoints: []EndpointState{{Name: "dynamic", IP: "203.0.113.7", Healthy: true}},
			Published: []string{"203.0.113.7"},
			Previous:  []string{"203.0.113.6"},
		})
		assert.Equal(t, "app.example.com now points to 203.0.113.7", m.title)
		assert.Contains(t, m.body, "Now: 203.0.113.7\n")
		assert.Contains(t, m.body, "Before: 203.0.113.6\n")
		assert.Contains(t, m.body, "  dynamic (203.0.113.7): healthy\n")
	})

	t.Run("update to a CNAME fails", func(t *testing.T) {
		m := newMessage(Event{
			Type:      config.EventDNSUpdateFailed,
			Domain:    "app.example.com",
			Endpoints: endpoints,
			Published: []string{"0a1b2c3d.cfargotunnel.com"},
			Previous:  []string{"192.0.2.10"},
			Error:     "boom",
		})
		assert.Equal(t, "DNS update failed for app.example.com", m.title)
		assert.Contains(t, m.body, "Wanted: 0a1b2c3d.cfargotunnel.com\n")
	})
}
