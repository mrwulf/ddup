package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	configkit "github.com/italypaleale/go-kit/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fork extension: notifications.detail is on unless the configuration turns it off
func TestNotificationsDetail(t *testing.T) {
	load := func(t *testing.T, extra string) *Config {
		t.Helper()

		configYAML := `domains:
  - recordName: "app.example.com"
    provider: "cf"
    endpoints:
      - url: "https://192.0.2.10/health"
        ip: "192.0.2.10"
providers:
  cf:
    cloudflare:
      apiToken: "token"
      zoneId: "zone"
notifications:
  ntfy:
    - name: "alerts"
      url: "https://ntfy.example.com/ddup"
` + extra
		cfgPath := filepath.Join(t.TempDir(), "config.yaml")
		err := os.WriteFile(cfgPath, []byte(configYAML), 0o600)
		require.NoError(t, err)
		t.Setenv("DDUP_CONFIG", cfgPath)

		cfg := GetDefaultConfig()
		err = configkit.LoadConfig(cfg, configkit.LoadConfigOpts{
			EnvVar:  "DDUP_CONFIG",
			DirName: "ddup",
		})
		require.NoError(t, err)
		err = cfg.Validate(slog.Default())
		require.NoError(t, err)
		return cfg
	}

	t.Run("default is on", func(t *testing.T) {
		cfg := load(t, "")
		assert.True(t, cfg.Notifications.Detail)
	})

	t.Run("opt out", func(t *testing.T) {
		cfg := load(t, "  detail: false\n")
		assert.False(t, cfg.Notifications.Detail)
	})
}
