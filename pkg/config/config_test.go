package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "sigs.k8s.io/yaml/goyaml.v3"
)

func TestValidateEndpointIP(t *testing.T) {
	tests := []struct {
		name      string
		ip        string
		expected  string
		expectErr bool
	}{
		{name: "IPv4", ip: "192.0.2.1", expected: "192.0.2.1"},
		{name: "IPv6 is canonicalized", ip: "2001:0DB8:0:0:0:0:0:1", expected: "2001:db8::1"},
		{name: "IPv4-mapped IPv6 remains IPv6", ip: "::ffff:192.0.2.1", expected: "::ffff:192.0.2.1"},
		{name: "invalid", ip: "not-an-ip", expectErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Providers: map[string]ConfigProvider{
					"test": {
						Cloudflare: &CloudflareConfig{},
					},
				},
				Domains: []ConfigDomain{
					{
						RecordName: "test.example.com",
						Provider:   "test",
						Endpoints: []*ConfigEndpoint{
							{
								URL: "https://test.example.com/health",
								IP:  tc.ip,
							},
						},
					},
				},
			}

			err := cfg.Validate(slog.Default())
			if tc.expectErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), `IP "not-an-ip" is not a valid IPv4 or IPv6 address`)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.expected, cfg.Domains[0].Endpoints[0].IP)
		})
	}
}

func TestSecretString(t *testing.T) {
	t.Setenv("DDUP_TEST_TOKEN", "from-env")
	secretFile := filepath.Join(t.TempDir(), "token")
	require.NoError(t, os.WriteFile(secretFile, []byte("from-file\n"), 0o600))

	var cfg Config
	dec := yaml.NewDecoder(strings.NewReader(`
providers:
  literal:
    cloudflare: {apiToken: "plain", zoneId: "zone"}
  env:
    cloudflare: {apiToken: !env DDUP_TEST_TOKEN, zoneId: "zone"}
  file:
    cloudflare: {apiToken: !file ` + secretFile + `, zoneId: "zone"}
domains:
  - recordName: !env DDUP_TEST_TOKEN
    provider: env
`))
	dec.KnownFields(true)
	require.NoError(t, dec.Decode(&cfg))

	assert.Equal(t, "plain", cfg.Providers["literal"].Cloudflare.APIToken.String())
	assert.Equal(t, "from-env", cfg.Providers["env"].Cloudflare.APIToken.String())
	assert.Equal(t, "from-file", cfg.Providers["file"].Cloudflare.APIToken.String(), "trailing newline is removed")
	assert.Equal(t, "from-env", cfg.Domains[0].RecordName.String())

	for _, bad := range []string{
		"apiToken: !env DDUP_TEST_NOT_SET_ANYWHERE",
		"apiToken: !env",
		"apiToken: !file /nonexistent/ddup/secret",
	} {
		var c ConfigProvider
		dec = yaml.NewDecoder(strings.NewReader("cloudflare: {" + bad + "}"))
		dec.KnownFields(true)
		require.Error(t, dec.Decode(&c), bad)
	}
}
