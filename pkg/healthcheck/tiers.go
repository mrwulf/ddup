package healthcheck

import (
	"github.com/italypaleale/ddup/pkg/config"
	"github.com/italypaleale/ddup/pkg/dns"
)

// publication is what should be in DNS for a set of healthy endpoints
type publication struct {
	// Targets that should be published: the healthy endpoints with the lowest priority value
	targets []dns.Target
	// Priority of the endpoints that are published
	priority int
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

func (p publication) values() []string {
	values := make([]string, len(p.targets))
	for i, t := range p.targets {
		values[i] = t.Value
	}
	return values
}

// selectPublication picks what to publish for the healthy targets (IPs or CNAME hostnames)
// The selection is selectLowestPriority; the targets are then given the proxied setting of their endpoint
// Targets that don't match a known endpoint count as priority 0 and are not proxied
func selectPublication(endpoints map[string]*config.ConfigEndpoint, healthy []string) publication {
	priorities := make(map[string]int, len(endpoints))
	for target, ep := range endpoints {
		priorities[target] = ep.Priority
	}

	selected := selectLowestPriority(priorities, healthy)
	pub := publication{
		targets: make([]dns.Target, len(selected)),
	}
	for i, value := range selected {
		pub.targets[i].Value = value
		ep := endpoints[value]
		if ep != nil {
			pub.targets[i].Proxied = ep.Proxied
			pub.priority = ep.Priority
		}
	}
	return pub
}
