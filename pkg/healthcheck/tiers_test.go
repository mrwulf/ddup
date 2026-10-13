package healthcheck

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/italypaleale/ddup/pkg/config"
	"github.com/italypaleale/ddup/pkg/dns"
)

func TestSelectPublication(t *testing.T) {
	endpoints := map[string]*config.ConfigEndpoint{
		"1.1.1.1":            {IP: "1.1.1.1", Proxied: true},
		"2.2.2.2":            {IP: "2.2.2.2", Proxied: true},
		"tunnel.example.com": {CNAME: "tunnel.example.com", Priority: 1},
	}

	t.Run("preferred priority only, with the proxied setting", func(t *testing.T) {
		pub := selectPublication(endpoints, []string{"tunnel.example.com", "1.1.1.1", "2.2.2.2"})
		assert.Equal(t, 0, pub.priority)
		assert.Equal(t, []dns.Target{{Value: "1.1.1.1", Proxied: true}, {Value: "2.2.2.2", Proxied: true}}, pub.targets)
	})

	t.Run("falls back to the CNAME", func(t *testing.T) {
		pub := selectPublication(endpoints, []string{"tunnel.example.com"})
		assert.Equal(t, 1, pub.priority)
		assert.Equal(t, []dns.Target{{Value: "tunnel.example.com"}}, pub.targets)
	})

	t.Run("nothing healthy", func(t *testing.T) {
		pub := selectPublication(endpoints, nil)
		assert.Equal(t, 0, pub.priority)
		assert.Empty(t, pub.targets)
	})

	t.Run("unknown targets count as priority 0 and are not proxied", func(t *testing.T) {
		pub := selectPublication(endpoints, []string{"tunnel.example.com", "9.9.9.9"})
		assert.Equal(t, []dns.Target{{Value: "9.9.9.9"}}, pub.targets)
	})
}
