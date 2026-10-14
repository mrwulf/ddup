package dns

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configkit "github.com/italypaleale/go-kit/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/italypaleale/ddup/pkg/config"
	"github.com/italypaleale/ddup/pkg/notify"
)

// These tests exercise the features of the open pull requests together:
// cname and providerOptions (typed targets), ipLookupFamily (dynamic DNS) and notifications

// loadIntegrationConfig loads a config the same way as the app does, which rejects unknown keys, then validates it as main does
func loadIntegrationConfig(t *testing.T, yaml string) (*config.Config, error) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	err := os.WriteFile(path, []byte(yaml), 0o600)
	require.NoError(t, err)
	t.Setenv("DDUP_CONFIG", path)

	cfg := config.GetDefaultConfig()
	err = configkit.LoadConfig(cfg, configkit.LoadConfigOpts{
		EnvVar:  "DDUP_CONFIG",
		DirName: "ddup",
	})
	require.NoError(t, err)

	err = cfg.Validate(slog.Default())
	if err != nil {
		return cfg, fmt.Errorf("validating the config: %w", err)
	}
	return cfg, ValidateConfig(cfg)
}

const integrationNotifications = `
notifications:
  webhooks:
    - name: "chat"
      format: "slack"
      url: "https://hooks.slack.com/services/example"
`

// integrationConfig returns a config with a mixed domain, and the provider block and the extra endpoint options passed
func integrationConfig(providerBlock, cnameOptions, extraEndpoints string) string {
	return `
ipLookupURLs:
  - url: "https://ip.example.com/v4"
    family: 4
domains:
  - recordName: "app.example.com"
    provider: "p"
    endpoints:
      - name: "fixed"
        url: "https://fixed.example.com/health"
        ip: "192.0.2.10"
      - name: "dynamic"
        ipLookupFamily: 4
      - name: "tunnel"
        url: "https://tunnel-health.example.com/health"
        cname: "0a1b2c3d.cfargotunnel.com"
        priority: 1
` + cnameOptions + extraEndpoints + `
providers:
  p:
` + providerBlock + integrationNotifications
}

const (
	cloudflareBlock = `    cloudflare:
      apiToken: "token"
      zoneId: "zone"
`
	proxiedOptions = `        providerOptions:
          cloudflare:
            proxied: true
`
)

func TestIntegrationMixedDomainValidates(t *testing.T) {
	t.Run("ip, dynamic and proxied cname in one domain", func(t *testing.T) {
		cfg, err := loadIntegrationConfig(t, integrationConfig(cloudflareBlock, proxiedOptions, ""))
		require.NoError(t, err)

		eps := cfg.Domains[0].Endpoints
		require.Len(t, eps, 3)
		assert.False(t, eps[0].Dynamic())
		assert.True(t, eps[1].Dynamic())
		assert.True(t, eps[2].Proxied())
		assert.Equal(t, "0a1b2c3d.cfargotunnel.com", eps[2].Target())

		// Two dynamic endpoints are not the "same target", even though neither has a fixed one
		_, err = loadIntegrationConfig(t, integrationConfig(cloudflareBlock, proxiedOptions, `      - name: "dynamic-2"
        ipLookupFamily: 4
        priority: 2
      - name: "dynamic-3"
        ipLookupFamily: 4
        priority: 2
`))
		require.NoError(t, err)
	})

	t.Run("a cname alone in its priority with dynamic endpoints around", func(t *testing.T) {
		_, err := loadIntegrationConfig(t, integrationConfig(cloudflareBlock, proxiedOptions, `      - name: "dynamic-after"
        ipLookupFamily: 4
        priority: 2
`))
		require.NoError(t, err)
	})

	t.Run("a cname can't share its priority with a dynamic endpoint", func(t *testing.T) {
		_, err := loadIntegrationConfig(t, integrationConfig(cloudflareBlock, proxiedOptions, `      - name: "dynamic-same-priority"
        ipLookupFamily: 4
        priority: 1
        providerOptions:
          cloudflare:
            proxied: true
`))
		require.ErrorContains(t, err, "a CNAME record can't coexist with other records")
	})

	t.Run("proxied must be the same for the dynamic and fixed endpoints of a priority", func(t *testing.T) {
		_, err := loadIntegrationConfig(t, integrationConfig(cloudflareBlock, proxiedOptions, `      - name: "dynamic-proxied"
        ipLookupFamily: 4
        providerOptions:
          cloudflare:
            proxied: true
`))
		require.ErrorContains(t, err, "must all use the same value for providerOptions.cloudflare.proxied")
	})

	t.Run("exactly one of ip, cname and ipLookupFamily", func(t *testing.T) {
		both := func(extra string) string {
			return integrationConfig(cloudflareBlock, proxiedOptions, `      - name: "bad"
        url: "https://bad.example.com/health"
        priority: 3
`+extra)
		}
		_, err := loadIntegrationConfig(t, both(`        ipLookupFamily: 4
        cname: "bad.example.net"
`))
		require.ErrorContains(t, err, "exactly one of ip, cname or ipLookupFamily must be set")

		_, err = loadIntegrationConfig(t, both(`        ipLookupFamily: 4
        ip: "192.0.2.50"
`))
		require.ErrorContains(t, err, "exactly one of ip, cname or ipLookupFamily must be set")

		_, err = loadIntegrationConfig(t, both(`        ip: "192.0.2.50"
        cname: "bad.example.net"
`))
		require.ErrorContains(t, err, "exactly one of ip, cname or ipLookupFamily must be set")

		_, err = loadIntegrationConfig(t, both(""))
		require.ErrorContains(t, err, "exactly one of ip, cname or ipLookupFamily must be set")
	})

	t.Run("ipLookupURLs is rejected on a cname endpoint", func(t *testing.T) {
		_, err := loadIntegrationConfig(t, integrationConfig(cloudflareBlock, proxiedOptions, `      - name: "bad"
        url: "https://bad.example.com/health"
        cname: "bad.example.net"
        priority: 3
        ipLookupURLs:
          - url: "https://ip.example.com/v4"
`))
		require.ErrorContains(t, err, "ipLookupURLs is set, but the endpoint has no ipLookupFamily")
	})
}

func TestIntegrationProviderCapabilities(t *testing.T) {
	// The notifications are configured in all cases
	providers := map[string]string{
		"azure":   "    azure:\n      subscriptionId: \"s\"\n      resourceGroupName: \"r\"\n      zoneName: \"example.com\"\n",
		"ovh":     "    ovh:\n      apiKey: \"k\"\n      apiSecret: \"s\"\n      consumerKey: \"c\"\n      zoneName: \"example.com\"\n",
		"route53": "    awsRoute53:\n      hostedZoneId: \"Z\"\n",
		"google":  "    googleCloudDns:\n      projectId: \"p\"\n      managedZone: \"z\"\n",
	}

	for name, block := range providers {
		t.Run(name+" rejects cname", func(t *testing.T) {
			_, err := loadIntegrationConfig(t, integrationConfig(block, "", ""))
			require.ErrorContains(t, err, "cname is not supported by provider 'p'")
		})

		t.Run(name+" rejects providerOptions.cloudflare", func(t *testing.T) {
			// Same domain without the cname, so the options are the only problem
			yaml := `
ipLookupURLs:
  - url: "https://ip.example.com/v4"
    family: 4
domains:
  - recordName: "app.example.com"
    provider: "p"
    endpoints:
      - name: "fixed"
        url: "https://fixed.example.com/health"
        ip: "192.0.2.10"
` + proxiedOptions + `      - name: "dynamic"
        ipLookupFamily: 4
        priority: 1
` + proxiedOptions + `
providers:
  p:
` + block + integrationNotifications
			_, err := loadIntegrationConfig(t, yaml)
			require.ErrorContains(t, err, "providerOptions.cloudflare can only be used with a cloudflare provider")
		})

		t.Run(name+" accepts ip and dynamic endpoints", func(t *testing.T) {
			yaml := `
ipLookupURLs:
  - url: "https://ip.example.com/v4"
    family: 4
domains:
  - recordName: "app.example.com"
    provider: "p"
    endpoints:
      - name: "fixed"
        url: "https://fixed.example.com/health"
        ip: "192.0.2.10"
      - name: "dynamic"
        ipLookupFamily: 4
        priority: 1
providers:
  p:
` + block + integrationNotifications
			_, err := loadIntegrationConfig(t, yaml)
			require.NoError(t, err)
		})
	}

	t.Run("cloudflare accepts everything", func(t *testing.T) {
		_, err := loadIntegrationConfig(t, integrationConfig(cloudflareBlock, proxiedOptions, ""))
		require.NoError(t, err)
	})
}

// The sample config must contain an example of every feature, load, validate, and be usable to start the app
func TestIntegrationSampleConfigHasEveryFeature(t *testing.T) {
	samplePath := filepath.Join("..", "..", "config.sample.yaml")
	t.Setenv("DDUP_CONFIG", samplePath)

	cfg := config.GetDefaultConfig()
	err := configkit.LoadConfig(cfg, configkit.LoadConfigOpts{
		EnvVar:  "DDUP_CONFIG",
		DirName: "ddup",
	})
	require.NoError(t, err)
	require.NoError(t, cfg.Validate(slog.Default()))
	require.NoError(t, ValidateConfig(cfg))

	var (
		hasIP, hasIPv6, hasCNAME, hasProxied, hasPriority, hasDynamic, hasOwnLookupURLs bool
		hasRecoverAfter, hasMethod, hasExpectStatus                                     bool
	)
	for _, d := range cfg.Domains {
		hasRecoverAfter = hasRecoverAfter || d.HealthChecks.RecoverAfter > 0
		hasMethod = hasMethod || d.HealthChecks.Method != ""
		hasExpectStatus = hasExpectStatus || d.HealthChecks.ExpectStatus != ""
		for _, e := range d.Endpoints {
			hasIP = hasIP || e.IP != ""
			hasIPv6 = hasIPv6 || strings.Contains(e.IP, ":")
			hasCNAME = hasCNAME || e.CNAME != ""
			hasProxied = hasProxied || e.Proxied()
			hasPriority = hasPriority || e.Priority > 0
			hasDynamic = hasDynamic || e.Dynamic()
			hasOwnLookupURLs = hasOwnLookupURLs || len(e.IPLookupURLs) > 0
		}
	}
	assert.True(t, hasIP, "fixed ip")
	assert.True(t, hasIPv6, "IPv6 ip")
	assert.True(t, hasCNAME, "cname")
	assert.True(t, hasProxied, "providerOptions.cloudflare.proxied")
	assert.True(t, hasPriority, "priority")
	assert.True(t, hasDynamic, "ipLookupFamily")
	assert.True(t, hasOwnLookupURLs, "ipLookupURLs on an endpoint")
	assert.True(t, hasRecoverAfter, "healthChecks.recoverAfter")
	assert.True(t, hasMethod, "healthChecks.method")
	assert.True(t, hasExpectStatus, "healthChecks.expectStatus")
	assert.NotEmpty(t, cfg.IPLookupURLs, "top-level ipLookupURLs")
	assert.NotEmpty(t, cfg.Notifications.Ntfy, "ntfy notifications")
	assert.NotEmpty(t, cfg.Notifications.Webhooks, "webhook notifications")
	assert.NotEmpty(t, cfg.Notifications.Email, "email notifications")
	assert.True(t, cfg.Server.Enabled, "server")

	// The notifier can be created from the sample, as main does at startup
	notifier, err := notify.New(t.Context(), cfg)
	require.NoError(t, err)
	require.NotNil(t, notifier)
	require.NoError(t, notifier.Shutdown(t.Context()))
}
