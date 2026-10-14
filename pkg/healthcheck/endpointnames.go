package healthcheck

import (
	"maps"

	"github.com/italypaleale/ddup/pkg/config"
	"github.com/italypaleale/ddup/pkg/healthcheck/checker"
)

// endpointNames returns the name of each endpoint worth showing, by target (the IP address or the CNAME hostname)
// Endpoints that look up their IP address are added when the address is found
func endpointNames(endpoints []*config.ConfigEndpoint) map[string]string {
	res := make(map[string]string, len(endpoints))
	for _, ep := range endpoints {
		if ep.Dynamic() {
			continue
		}
		res[ep.Target()] = displayName(ep)
	}
	return res
}

// displayName returns the name of the endpoint, or an empty string if it's just the default (its URL), which is not worth showing
func displayName(ep *config.ConfigEndpoint) string {
	if ep.Name == ep.URL {
		return ""
	}
	return ep.Name
}

// noteNames records the name of the endpoints that look up their IP address, under the address they found
// Addresses that are not in the results or in the keep lists are forgotten, so the map doesn't grow when the address changes
func (dc *domainChecker) noteNames(results []checker.Result, keep ...[]string) {
	dc.lock.Lock()
	defer dc.lock.Unlock()

	if dc.dynamicNames == nil {
		dc.dynamicNames = make(map[string]string)
	}

	keepSet := make(map[string]struct{})
	for _, list := range keep {
		for _, ip := range list {
			keepSet[ip] = struct{}{}
		}
	}
	for _, r := range results {
		ip := resultIP(r)
		if ip == "" {
			continue
		}
		keepSet[ip] = struct{}{}
		if r.Endpoint.Dynamic() {
			dc.dynamicNames[ip] = displayName(r.Endpoint)
		}
	}

	maps.DeleteFunc(dc.dynamicNames, func(ip string, _ string) bool {
		_, ok := keepSet[ip]
		return !ok
	})
}

// getName returns the name of the endpoint with an address, or an empty string if it has none
func (dc *domainChecker) getName(ip string) string {
	dc.lock.Lock()
	defer dc.lock.Unlock()

	name, ok := dc.names[ip]
	if ok {
		return name
	}
	return dc.dynamicNames[ip]
}
