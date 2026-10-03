package healthcheck

import (
	"github.com/italypaleale/ddup/pkg/config"
	"github.com/italypaleale/ddup/pkg/dns"
)

// publication is what should be in DNS for a set of healthy endpoints
type publication struct {
	// Targets that should be published
	targets []dns.Target
}

// keys returns strings that identify the targets, including whether they're proxied, so publications can be compared
func (p publication) keys() []string {
	keys := make([]string, len(p.targets))
	for i, t := range p.targets {
		keys[i] = t.Value
		if t.Proxied {
			keys[i] += "|proxied"
		}
	}
	return keys
}

// selectPublication builds the targets to publish for the healthy IP addresses
// A target that doesn't match a known endpoint is not proxied
func selectPublication(endpoints map[string]*config.ConfigEndpoint, healthy []string) publication {
	pub := publication{targets: make([]dns.Target, len(healthy))}
	for i, value := range healthy {
		pub.targets[i] = dns.Target{Value: value}
		ep := endpoints[value]
		if ep != nil {
			pub.targets[i].Proxied = ep.Proxied
		}
	}
	return pub
}
