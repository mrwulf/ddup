package notify

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/italypaleale/ddup/pkg/config"
)

func TestProviderSecrets(t *testing.T) {
	secrets := providerSecrets(config.ConfigProvider{
		Cloudflare: &config.CloudflareConfig{APIToken: "cf-token", ZoneID: "zone-id"},
		OVH:        &config.OVHConfig{APIKey: "ovh-key", APISecret: "ovh-secret", ConsumerKey: "ovh-consumer", ZoneName: "example.com"},
	})
	assert.ElementsMatch(t, []string{"cf-token", "ovh-key", "ovh-secret", "ovh-consumer"}, secrets)
}

func TestForkRedactor(t *testing.T) {
	cfg := &config.Config{
		Providers: map[string]config.ConfigProvider{
			"cf": {Cloudflare: &config.CloudflareConfig{APIToken: "cf-token", ZoneID: "zone-id"}},
		},
		Notifications: config.ConfigNotifications{
			Webhooks: []config.ConfigNotificationWebhook{
				{URL: "https://ntfy.example.com/topic-secret", Token: "ntfy-token"},
			},
			Email: []config.ConfigNotificationEmail{
				//nolint:gosec // Fake values for a test
				{ConnectionString: "smtp://user:smtp-password@mail.example.com:587"},
			},
		},
	}

	r := newForkRedactor(cfg)
	for _, secret := range []string{"cf-token", "ntfy-token", "smtp-password"} {
		assert.NotContains(t, r.String("failed with "+secret), secret)
	}
	assert.Contains(t, r.String("zone-id is not a secret"), "zone-id")
}
