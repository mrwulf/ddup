package config

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateLeaderElection(t *testing.T) {
	newConfig := func(le ConfigLeaderElection) *Config {
		cfg := &Config{
			Providers: map[string]ConfigProvider{"test": {Cloudflare: &CloudflareConfig{}}},
			Domains: []ConfigDomain{{
				RecordName: "test.example.com",
				Provider:   "test",
				Endpoints:  []*ConfigEndpoint{{URL: "https://a.example.com", IP: "192.0.2.1"}},
			}},
			LeaderElection: le,
		}
		return cfg
	}

	t.Run("disabled needs nothing", func(t *testing.T) {
		cfg := newConfig(ConfigLeaderElection{})
		require.NoError(t, cfg.Validate(nil))
		assert.Empty(t, cfg.LeaderElection.Name, "defaults are not applied when it is off")
	})

	t.Run("defaults", func(t *testing.T) {
		cfg := newConfig(ConfigLeaderElection{Enabled: true})
		require.NoError(t, cfg.Validate(nil))
		assert.Equal(t, ConfigLeaderElection{
			Enabled:       true,
			Type:          "kubernetes",
			Name:          "ddup",
			LeaseDuration: 15 * time.Second,
			RenewDeadline: 10 * time.Second,
			RetryPeriod:   2 * time.Second,
		}, cfg.LeaderElection)
	})

	t.Run("unknown type", func(t *testing.T) {
		cfg := newConfig(ConfigLeaderElection{Enabled: true, Type: "etcd"})
		require.ErrorContains(t, cfg.Validate(nil), "leaderElection.type must be 'kubernetes'")
	})

	t.Run("durations must be ordered", func(t *testing.T) {
		cfg := newConfig(ConfigLeaderElection{Enabled: true, LeaseDuration: 5 * time.Second, RenewDeadline: 5 * time.Second})
		require.ErrorContains(t, cfg.Validate(nil), "leaseDuration > renewDeadline > retryPeriod")

		cfg = newConfig(ConfigLeaderElection{Enabled: true, RenewDeadline: 2 * time.Second, RetryPeriod: 2 * time.Second})
		require.ErrorContains(t, cfg.Validate(nil), "leaseDuration > renewDeadline > retryPeriod")
	})
}
