package healthcheck

import (
	"errors"
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/italypaleale/ddup/pkg/config"
	"github.com/italypaleale/ddup/pkg/healthcheck/checker"
	"github.com/italypaleale/ddup/pkg/notify"
)

func TestCheckErrorText(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		assert.Empty(t, checkErrorText(nil))
	})

	t.Run("URL of the endpoint is removed", func(t *testing.T) {
		urlErr := &url.Error{
			Op:  "Get",
			URL: "https://192.0.2.10/health/secret-path?token=abc123",
			Err: errors.New("dial tcp 192.0.2.10:443: connect: connection refused"),
		}
		err := fmt.Errorf("HTTP request failed: %w", urlErr)
		assert.Equal(t, "HTTP request failed: dial tcp 192.0.2.10:443: connect: connection refused", checkErrorText(err))
	})

	t.Run("credentials in other URLs are removed", func(t *testing.T) {
		err := errors.New("looking up the IP address: https://user:pass@lookup.example.com/ip: status code 500")
		assert.Equal(t, "looking up the IP address: https://[redacted]@lookup.example.com/ip: status code 500", checkErrorText(err))
	})

	t.Run("other errors are kept", func(t *testing.T) {
		assert.Equal(t, "status code 503", checkErrorText(errors.New("status code 503")))
	})
}

func TestNewNotifyDetail(t *testing.T) {
	results := []checker.Result{
		{
			Endpoint: &config.ConfigEndpoint{Name: "primary", IP: "192.0.2.10"},
			Healthy:  true,
		},
		{
			Endpoint: &config.ConfigEndpoint{Name: "secondary", IP: "192.0.2.11"},
			Error:    errors.New("status code 503"),
		},
		{
			Endpoint: &config.ConfigEndpoint{Name: "backup", IP: "192.0.2.20", Priority: 1},
			Healthy:  true,
		},
	}
	priorities := map[string]int{
		"192.0.2.10": 0,
		"192.0.2.11": 0,
		"192.0.2.20": 1,
	}

	d := newNotifyDetail(results, priorities, []string{"192.0.2.10"})
	assert.Equal(t, notify.Detail{
		Endpoints: map[string]notify.EndpointDetail{
			"192.0.2.10": {
				Priority: 0,
				Active:   true,
			},
			"192.0.2.11": {
				Priority: 0,
				Error:    "status code 503",
			},
			"192.0.2.20": {
				Priority: 1,
			},
		},
	}, d)
}

func TestNotifyAddsDetail(t *testing.T) {
	rec := &recordingNotifier{}
	hc := &HealthChecker{
		domainCheckers: map[string]*domainChecker{
			"example.com": {
				notifyDetail: notify.Detail{
					Endpoints: map[string]notify.EndpointDetail{
						"192.0.2.10": {
							Active: true,
						},
					},
				},
			},
		},
	}
	hc.SetNotifier(rec)

	hc.notify(notify.Event{
		Type:   config.EventDNSUpdated,
		Domain: "example.com",
	})
	hc.notify(notify.Event{
		Type:   config.EventDNSUpdated,
		Domain: "other.example.com",
	})

	events := rec.Events()
	assert.Len(t, events, 2)
	assert.True(t, events[0].Detail.Endpoints["192.0.2.10"].Active)
	assert.Empty(t, events[1].Detail.Endpoints)
}
