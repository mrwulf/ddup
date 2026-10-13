package config

import (
	"os"
	"path/filepath"
	"testing"

	configkit "github.com/italypaleale/go-kit/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Secrets can be read from files with the tags of go-kit's config package
func TestLoadFileReferences(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "token"), []byte("from-file\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "secrets.env"), []byte("ZONE=zone-from-env-file\nHOOK=https://hooks.example.com/x\n"), 0o600))

	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(`
domains:
  - recordName: !envfile secrets.env RECORD
    provider: p
    endpoints:
      - url: "https://a.example.com/health"
        ip: "192.0.2.1"
providers:
  p:
    cloudflare:
      apiToken: !file token
      zoneId: !envfile secrets.env ZONE
webhooks:
  - url: !envfile secrets.env HOOK
    headers:
      Authorization: "Bearer {{ .Vars.token }}"
    vars:
      token: !file token
`), 0o600))
	t.Setenv("DDUP_TEST_CONFIG", cfgPath)

	// The record name isn't in the dotenv file, so loading must fail and name the variable but not the contents
	cfg := GetDefaultConfig()
	err := configkit.LoadConfig(cfg, configkit.LoadConfigOpts{EnvVar: "DDUP_TEST_CONFIG", DirName: "ddup"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "RECORD")
	assert.NotContains(t, err.Error(), "zone-from-env-file")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "secrets.env"), []byte("ZONE=zone-from-env-file\nHOOK=https://hooks.example.com/x\nRECORD=home.example.com\n"), 0o600))
	cfg = GetDefaultConfig()
	err = configkit.LoadConfig(cfg, configkit.LoadConfigOpts{EnvVar: "DDUP_TEST_CONFIG", DirName: "ddup"})
	require.NoError(t, err)

	assert.Equal(t, "from-file", cfg.Providers["p"].Cloudflare.APIToken, "trailing newline is removed")
	assert.Equal(t, "zone-from-env-file", cfg.Providers["p"].Cloudflare.ZoneID)
	assert.Equal(t, "home.example.com", cfg.Domains[0].RecordName)
	assert.Equal(t, "https://hooks.example.com/x", cfg.Webhooks[0].URL)
	assert.Equal(t, "from-file", cfg.Webhooks[0].Vars["token"])
}
