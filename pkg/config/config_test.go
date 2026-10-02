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

func TestStatusMatcher(t *testing.T) {
	m, err := ParseStatusMatcher([]string{"2xx", "301-302", "418"})
	require.NoError(t, err)
	for _, code := range []int{200, 299, 301, 302, 418} {
		assert.True(t, m.Match(code), code)
	}
	for _, code := range []int{199, 300, 303, 404, 500} {
		assert.False(t, m.Match(code), code)
	}

	assert.True(t, StatusMatcher{}.Match(204), "zero value uses default")
	for _, bad := range []string{"", "abc", "6xx", "299-200", "99", "600"} {
		_, err = ParseStatusMatcher([]string{bad})
		require.Error(t, err, bad)
	}
}

func TestValidateWebhooks(t *testing.T) {
	cfg := &Config{Webhooks: []ConfigWebhook{{URL: "https://ntfy.example.com/topic", Events: []string{"dns_updated"}, Body: "{{ .Domain }}"}}}
	require.NoError(t, cfg.validateWebhooks())
	assert.Equal(t, "ntfy.example.com", cfg.Webhooks[0].Name)
	assert.Equal(t, "POST", cfg.Webhooks[0].Method)
	assert.Equal(t, 5, cfg.Webhooks[0].Attempts)

	for name, w := range map[string]ConfigWebhook{
		"bad url":      {URL: "ftp://x"},
		"bad event":    {URL: "https://x.example.com", Events: []string{"nope"}},
		"bad template": {URL: "https://x.example.com", Body: "{{ .Domain"},
		"bad header":   {URL: "https://x.example.com", Headers: map[string]SecretString{"X": "{{"}},
	} {
		cfg = &Config{Webhooks: []ConfigWebhook{w}}
		require.Error(t, cfg.validateWebhooks(), name)
	}
}

func TestValidateWebhooks_TemplateFuncs(t *testing.T) {
	cfg := &Config{Webhooks: []ConfigWebhook{{URL: "https://x.example.com", Body: `{{ join .Healthy ", " }} {{ json .Endpoints }}`}}}
	require.NoError(t, cfg.validateWebhooks())
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
webhooks:
  - url: !env DDUP_TEST_TOKEN
    headers:
      Authorization: !env DDUP_TEST_TOKEN
`))
	dec.KnownFields(true)
	require.NoError(t, dec.Decode(&cfg))

	assert.Equal(t, "plain", cfg.Providers["literal"].Cloudflare.APIToken.String())
	assert.Equal(t, "from-env", cfg.Providers["env"].Cloudflare.APIToken.String())
	assert.Equal(t, "from-file", cfg.Providers["file"].Cloudflare.APIToken.String(), "trailing newline is removed")
	assert.Equal(t, "from-env", cfg.Webhooks[0].URL.String())
	assert.Equal(t, "from-env", cfg.Webhooks[0].Headers["Authorization"].String())

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

func TestValidateEndpoints_TiersAndTargets(t *testing.T) {
	cloudflare := ConfigProvider{Cloudflare: &CloudflareConfig{}}
	ovh := ConfigProvider{OVH: &OVHConfig{}}

	tests := []struct {
		name      string
		provider  ConfigProvider
		endpoints []*ConfigEndpoint
		errSubstr string
	}{
		{name: "ips with priorities", provider: cloudflare, endpoints: []*ConfigEndpoint{
			{URL: "https://a", IP: "1.1.1.1"}, {URL: "https://b", IP: "2.2.2.2"}, {URL: "https://c", IP: "3.3.3.3", Priority: 1},
		}},
		{name: "cname in its own priority", provider: cloudflare, endpoints: []*ConfigEndpoint{
			{URL: "https://a", IP: "1.1.1.1"}, {URL: "https://c", CNAME: "tunnel.example.com", Proxied: true, Priority: 1},
		}},
		{name: "ip and cname", provider: cloudflare, endpoints: []*ConfigEndpoint{{URL: "https://a", IP: "1.1.1.1", CNAME: "a.example.com"}}, errSubstr: "mutually exclusive"},
		{name: "neither ip nor cname", provider: cloudflare, endpoints: []*ConfigEndpoint{{URL: "https://a"}}, errSubstr: "one of ip and cname is required"},
		{name: "cname with other endpoints in the same priority", provider: cloudflare, endpoints: []*ConfigEndpoint{
			{URL: "https://a", IP: "1.1.1.1"}, {URL: "https://c", CNAME: "tunnel.example.com"},
		}, errSubstr: "can't coexist"},
		{name: "two cnames in the same priority", provider: cloudflare, endpoints: []*ConfigEndpoint{
			{URL: "https://a", CNAME: "a.example.com"}, {URL: "https://c", CNAME: "b.example.com"},
		}, errSubstr: "can't coexist"},
		{name: "mixed proxied in a priority", provider: cloudflare, endpoints: []*ConfigEndpoint{
			{URL: "https://a", IP: "1.1.1.1", Proxied: true}, {URL: "https://b", IP: "2.2.2.2"},
		}, errSubstr: "same value for proxied"},
		{name: "duplicate target", provider: cloudflare, endpoints: []*ConfigEndpoint{
			{URL: "https://a", IP: "1.1.1.1"}, {URL: "https://b", IP: "1.1.1.1", Priority: 1},
		}, errSubstr: "more than one endpoint"},
		{name: "negative priority", provider: cloudflare, endpoints: []*ConfigEndpoint{{URL: "https://a", IP: "1.1.1.1", Priority: -1}}, errSubstr: "must not be negative"},
		{name: "cname is an ip", provider: cloudflare, endpoints: []*ConfigEndpoint{{URL: "https://a", CNAME: "1.1.1.1"}}, errSubstr: "IP address"},
		{name: "invalid cname", provider: cloudflare, endpoints: []*ConfigEndpoint{{URL: "https://a", CNAME: "not a host"}}, errSubstr: "not a valid hostname"},
		{name: "cname needs cloudflare", provider: ovh, endpoints: []*ConfigEndpoint{{URL: "https://a", CNAME: "a.example.com"}}, errSubstr: "only supported by the Cloudflare provider"},
		{name: "proxied needs cloudflare", provider: ovh, endpoints: []*ConfigEndpoint{{URL: "https://a", IP: "1.1.1.1", Proxied: true}}, errSubstr: "only supported by the Cloudflare provider"},
		{name: "priorities work with any provider", provider: ovh, endpoints: []*ConfigEndpoint{
			{URL: "https://a", IP: "1.1.1.1"}, {URL: "https://b", IP: "2.2.2.2", Priority: 1},
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Providers: map[string]ConfigProvider{"p": tc.provider},
				Domains:   []ConfigDomain{{RecordName: "app.example.com", Provider: "p", Endpoints: tc.endpoints}},
			}
			err := cfg.Validate(slog.Default())
			if tc.errSubstr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errSubstr)
				return
			}
			require.NoError(t, err)
		})
	}

	t.Run("cname is normalized", func(t *testing.T) {
		cfg := &Config{
			Providers: map[string]ConfigProvider{"p": cloudflare},
			Domains:   []ConfigDomain{{RecordName: "app.example.com", Provider: "p", Endpoints: []*ConfigEndpoint{{URL: "https://a", CNAME: "Tunnel.Example.COM."}}}},
		}
		require.NoError(t, cfg.Validate(slog.Default()))
		assert.Equal(t, "tunnel.example.com", cfg.Domains[0].Endpoints[0].CNAME)
		assert.Equal(t, "tunnel.example.com", cfg.Domains[0].Endpoints[0].Target())
	})
}
