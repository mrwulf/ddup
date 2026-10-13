package config

import (
	"bytes"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	configkit "github.com/italypaleale/go-kit/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
		"bad header":   {URL: "https://x.example.com", Headers: map[string]string{"X": "{{"}},
	} {
		cfg = &Config{Webhooks: []ConfigWebhook{w}}
		require.Error(t, cfg.validateWebhooks(), name)
	}
}

func TestValidateWebhooks_TemplateFuncs(t *testing.T) {
	cfg := &Config{Webhooks: []ConfigWebhook{{URL: "https://x.example.com", Body: `{{ join .Healthy ", " }} {{ json .Endpoints }}`}}}
	require.NoError(t, cfg.validateWebhooks())
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
		{name: "neither ip nor cname", provider: cloudflare, endpoints: []*ConfigEndpoint{{URL: "https://a"}}, errSubstr: "one of ip, cname and ipLookup is required"},
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

func TestValidateEndpoints_IPLookup(t *testing.T) {
	cloudflare := ConfigProvider{Cloudflare: &CloudflareConfig{}}

	tests := []struct {
		name      string
		endpoint  *ConfigEndpoint
		errSubstr string
	}{
		{name: "lookup without a health check URL", endpoint: &ConfigEndpoint{IPLookup: &ConfigIPLookup{URLs: []string{"https://api.ipify.org"}}}},
		{name: "lookup with family and pattern", endpoint: &ConfigEndpoint{URL: "https://home.example.com/health", Proxied: true, IPLookup: &ConfigIPLookup{
			URLs: []string{"https://api6.ipify.org?format=json", "https://example.com/ip"}, Family: 6, Pattern: `"ip":"([^"]+)"`,
		}}},
		{name: "no URLs", endpoint: &ConfigEndpoint{IPLookup: &ConfigIPLookup{}}, errSubstr: "at least one URL"},
		{name: "relative URL", endpoint: &ConfigEndpoint{IPLookup: &ConfigIPLookup{URLs: []string{"/ip"}}}, errSubstr: "absolute http(s) URL"},
		{name: "bad family", endpoint: &ConfigEndpoint{IPLookup: &ConfigIPLookup{URLs: []string{"https://x.example.com"}, Family: 5}}, errSubstr: "family must be 4 or 6"},
		{name: "bad pattern", endpoint: &ConfigEndpoint{IPLookup: &ConfigIPLookup{URLs: []string{"https://x.example.com"}, Pattern: "("}}, errSubstr: "pattern is invalid"},
		{name: "pattern needs one group", endpoint: &ConfigEndpoint{IPLookup: &ConfigIPLookup{URLs: []string{"https://x.example.com"}, Pattern: `\d+`}}, errSubstr: "exactly one capture group"},
		{name: "lookup and ip", endpoint: &ConfigEndpoint{IP: "1.1.1.1", IPLookup: &ConfigIPLookup{URLs: []string{"https://x.example.com"}}}, errSubstr: "mutually exclusive"},
		{name: "lookup and cname", endpoint: &ConfigEndpoint{CNAME: "a.example.com", URL: "https://a", IPLookup: &ConfigIPLookup{URLs: []string{"https://x.example.com"}}}, errSubstr: "mutually exclusive"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Providers: map[string]ConfigProvider{"p": cloudflare},
				Domains:   []ConfigDomain{{RecordName: "home.example.com", Provider: "p", Endpoints: []*ConfigEndpoint{tc.endpoint}}},
			}
			err := cfg.Validate(slog.Default())
			if tc.errSubstr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errSubstr)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, tc.endpoint.Name, "name defaults to a URL")
			assert.True(t, tc.endpoint.Dynamic())
			assert.Empty(t, tc.endpoint.Target())
		})
	}

	t.Run("several dynamic endpoints don't collide", func(t *testing.T) {
		lookup := func() *ConfigIPLookup { return &ConfigIPLookup{URLs: []string{"https://api.ipify.org"}} }
		cfg := &Config{
			Providers: map[string]ConfigProvider{"p": cloudflare},
			Domains: []ConfigDomain{{RecordName: "home.example.com", Provider: "p", Endpoints: []*ConfigEndpoint{
				{IPLookup: lookup()}, {IPLookup: lookup(), Priority: 1},
			}}},
		}
		require.NoError(t, cfg.Validate(slog.Default()))
	})
}

func TestRedactURL(t *testing.T) {
	assert.Equal(t, "https://ip.example.com/ip", RedactURL("https://user:pass@ip.example.com/ip?token=s3cret#frag"))
	assert.Equal(t, "https://ip.example.com", RedactURL("https://ip.example.com"))
	assert.Equal(t, "(invalid URL)", RedactURL("not a url token=s3cret"))
}

func TestValidateEndpointPriority(t *testing.T) {
	tests := []struct {
		name      string
		priority  int
		expectErr string
	}{
		{name: "default", priority: 0},
		{name: "positive", priority: 1},
		{name: "large", priority: 1000},
		{name: "negative", priority: -1, expectErr: "domain test.example.com endpoint 1 is invalid: priority must not be negative"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Providers: map[string]ConfigProvider{"test": {Cloudflare: &CloudflareConfig{}}},
				Domains: []ConfigDomain{
					{
						RecordName: "test.example.com",
						Provider:   "test",
						Endpoints: []*ConfigEndpoint{
							{
								URL: "https://a.example.com/health",
								IP:  "192.0.2.1",
							},
							{
								URL:      "https://b.example.com/health",
								IP:       "192.0.2.2",
								Priority: tc.priority,
							},
						},
					},
				},
			}

			err := cfg.Validate(slog.Default())
			if tc.expectErr != "" {
				require.ErrorContains(t, err, tc.expectErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.priority, cfg.Domains[0].Endpoints[1].Priority)
		})
	}
}

func TestValidate(t *testing.T) {
	// Returns a valid config, which each test case changes
	newConfig := func() *Config {
		return &Config{
			Providers: map[string]ConfigProvider{"test": {Cloudflare: &CloudflareConfig{}}},
			Domains: []ConfigDomain{{
				RecordName: "test.example.com",
				Provider:   "test",
				Endpoints:  []*ConfigEndpoint{{URL: "https://test.example.com/health", IP: "192.0.2.1"}},
			}},
		}
	}

	errorTests := []struct {
		name      string
		mutate    func(c *Config)
		expectErr string
	}{
		{
			name:      "no providers",
			mutate:    func(c *Config) { c.Providers = nil },
			expectErr: "at least one provider must be configured",
		},
		{
			name:      "provider without a backend",
			mutate:    func(c *Config) { c.Providers["test"] = ConfigProvider{} },
			expectErr: "provider 'test' is invalid: exactly one provider must be configured",
		},
		{
			name: "provider with two backends",
			mutate: func(c *Config) {
				c.Providers["test"] = ConfigProvider{Cloudflare: &CloudflareConfig{}, OVH: &OVHConfig{}}
			},
			expectErr: "provider 'test' is invalid: exactly one provider must be configured",
		},
		{
			name:      "no domains",
			mutate:    func(c *Config) { c.Domains = nil },
			expectErr: "no domains configured",
		},
		{
			name:      "empty record name",
			mutate:    func(c *Config) { c.Domains[0].RecordName = "" },
			expectErr: "domain 0 is invalid: recordName is empty",
		},
		{
			name:      "no endpoints",
			mutate:    func(c *Config) { c.Domains[0].Endpoints = nil },
			expectErr: "domain test.example.com is invalid: endpoints list is empty",
		},
		{
			name:      "empty provider",
			mutate:    func(c *Config) { c.Domains[0].Provider = "" },
			expectErr: "domain 0 is invalid: provider is empty",
		},
		{
			name:      "unknown provider",
			mutate:    func(c *Config) { c.Domains[0].Provider = "other" },
			expectErr: "domain 0 is invalid: provider 'other' does not exist in the provider configuration",
		},
		{
			name:      "unsupported method",
			mutate:    func(c *Config) { c.Domains[0].HealthChecks.Method = "POST" },
			expectErr: "domain test.example.com is invalid: healthChecks.method must be GET or HEAD",
		},
		{
			name:      "negative recoverAfter",
			mutate:    func(c *Config) { c.Domains[0].HealthChecks.RecoverAfter = -1 },
			expectErr: "domain test.example.com is invalid: healthChecks.recoverAfter must not be negative",
		},
		{
			name:      "empty URL",
			mutate:    func(c *Config) { c.Domains[0].Endpoints[0].URL = "" },
			expectErr: "domain test.example.com endpoint 0 is invalid: URL is empty",
		},
		{
			name:      "empty IP",
			mutate:    func(c *Config) { c.Domains[0].Endpoints[0].IP = "" },
			expectErr: "domain test.example.com endpoint 0 is invalid: one of ip, cname and ipLookup is required",
		},
	}

	for _, tc := range errorTests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := newConfig()
			tc.mutate(cfg)
			err := cfg.Validate(slog.Default())
			require.ErrorContains(t, err, tc.expectErr)
		})
	}

	t.Run("valid config", func(t *testing.T) {
		require.NoError(t, newConfig().Validate(slog.Default()))
	})

	t.Run("defaults and sanitization", func(t *testing.T) {
		cfg := newConfig()
		cfg.Domains[0].HealthChecks.Method = "head"
		cfg.Domains[0].Endpoints = append(cfg.Domains[0].Endpoints, &ConfigEndpoint{Name: "named", URL: "https://b.example.com/health", IP: "192.0.2.2"})

		err := cfg.Validate(slog.Default())
		require.NoError(t, err)

		d := cfg.Domains[0]
		assert.Equal(t, 120, d.TTL)
		assert.Equal(t, http.MethodHead, d.HealthChecks.Method)
		// Endpoint names default to the URL
		assert.Equal(t, "https://test.example.com/health", d.Endpoints[0].Name)
		assert.Equal(t, "named", d.Endpoints[1].Name)
	})

	t.Run("configured TTL is kept", func(t *testing.T) {
		cfg := newConfig()
		cfg.Domains[0].TTL = 300

		err := cfg.Validate(slog.Default())
		require.NoError(t, err)
		assert.Equal(t, 300, cfg.Domains[0].TTL)
	})
}

func TestCountSetProperties(t *testing.T) {
	assert.Equal(t, 0, countSetProperties(ConfigProvider{}))
	assert.Equal(t, 1, countSetProperties(ConfigProvider{Azure: &AzureConfig{}}))
	assert.Equal(t, 2, countSetProperties(&ConfigProvider{Azure: &AzureConfig{}, OVH: &OVHConfig{}}))
	assert.Panics(t, func() {
		countSetProperties("not a struct")
	})
}

// The sample config documents the options, so it must load and pass validation
func TestSampleConfig(t *testing.T) {
	// The sample references secret files with !file: point them to temporary files, so the test does not depend on the machine
	sample, err := os.ReadFile(filepath.Join("..", "..", "config.sample.yaml"))
	require.NoError(t, err)
	secretsDir := t.TempDir()
	samplePath := filepath.Join(t.TempDir(), "config.yaml")
	for _, name := range []string{"email-api-key", "email-to"} {
		err = os.WriteFile(filepath.Join(secretsDir, name), []byte("test-value\n"), 0o600)
		require.NoError(t, err)
	}
	sample = bytes.ReplaceAll(sample, []byte("!file /run/secrets/"), []byte("!file "+secretsDir+"/"))
	err = os.WriteFile(samplePath, sample, 0o600)
	require.NoError(t, err)
	t.Setenv("DDUP_CONFIG", samplePath)

	// Load it the same way as the app, which rejects unknown keys
	cfg := GetDefaultConfig()
	err = configkit.LoadConfig(cfg, configkit.LoadConfigOpts{
		EnvVar:  "DDUP_CONFIG",
		DirName: "ddup",
	})
	require.NoError(t, err)
	assert.Equal(t, samplePath, cfg.GetLoadedConfigPath())

	err = cfg.Validate(slog.Default())
	require.NoError(t, err)
	assert.NotEmpty(t, cfg.Domains)
	assert.NotEmpty(t, cfg.Providers)
}
