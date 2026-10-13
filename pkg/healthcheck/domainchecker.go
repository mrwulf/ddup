package healthcheck

import (
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/italypaleale/ddup/pkg/config"
	"github.com/italypaleale/ddup/pkg/dns"
	"github.com/italypaleale/ddup/pkg/healthcheck/checker"
)

type domainChecker struct {
	lock       sync.Mutex
	checker    checker.Checker
	ttl        int
	healthyIPs []string
	failedIPs  map[string]int
	provider   dns.Provider
	// Endpoints by target (IP or CNAME hostname); may be nil, in which case all targets have priority 0 and are not proxied
	// Endpoints that look up their IP address are added when their address is found, and removed when it's not used any more
	endpoints   map[string]*config.ConfigEndpoint
	lastUpdated time.Time
	lastError   string

	// Consecutive successful checks for endpoints that were removed and are recovering
	recovering map[string]int
	// True if the last cycle found no healthy endpoints; used to notify only on transitions
	allDown bool
	// Last update error that webhooks were notified about; used to avoid repeating the same notification
	notifiedError string

	// When each endpoint last changed state and last failed, by target; in memory only
	times map[string]*endpointTimes
	// Returns the current time; nil means time.Now
	clock func() time.Time
}

// endpointTimes tracks the state changes of one endpoint
type endpointTimes struct {
	healthy bool
	// When the endpoint last changed state; for a new endpoint, when it was first checked
	since time.Time
	// True until the endpoint changes state for the first time, so `since` is only when ddup first saw it
	sinceStart bool
	// Last time a check failed; zero if never
	lastFailure time.Time
	// True if a check of the endpoint ran in the current cycle
	seen bool
}

func (dc *domainChecker) now() time.Time {
	if dc.clock != nil {
		return dc.clock()
	}
	return time.Now()
}

// recordCheck notes the outcome of a single check of an endpoint
func (dc *domainChecker) recordCheck(target string, healthy bool) {
	dc.lock.Lock()
	defer dc.lock.Unlock()

	if dc.times == nil {
		dc.times = make(map[string]*endpointTimes)
	}
	t := dc.times[target]
	if t == nil {
		t = &endpointTimes{}
		dc.times[target] = t
	}
	t.seen = true
	if !healthy {
		t.lastFailure = dc.now()
	}
}

// updateTimes records the state changes after a cycle, given the targets that are healthy now
// Endpoints that were not checked in this cycle are forgotten, unless they are still configured
func (dc *domainChecker) updateTimes(healthyIPs []string) {
	dc.lock.Lock()
	defer dc.lock.Unlock()

	now := dc.now()
	for target, t := range dc.times {
		if !t.seen {
			_, configured := dc.endpoints[target]
			if !configured {
				delete(dc.times, target)
			}
			continue
		}
		t.seen = false

		healthy := slices.Contains(healthyIPs, target)
		switch {
		case t.since.IsZero():
			t.healthy = healthy
			t.since = now
			t.sinceStart = true
		case t.healthy != healthy:
			t.healthy = healthy
			t.since = now
			t.sinceStart = false
		}
	}
}

// getTimes returns a copy of the times for an endpoint, or nil if there are none
func (dc *domainChecker) getTimes(target string) *endpointTimes {
	dc.lock.Lock()
	defer dc.lock.Unlock()

	t := dc.times[target]
	if t == nil || t.since.IsZero() {
		return nil
	}
	cp := *t
	return &cp
}

func (dc *domainChecker) getState() (healthyIPs []string, failedIPs map[string]int, lastUpdated time.Time, lastError string) {
	dc.lock.Lock()
	defer dc.lock.Unlock()

	return dc.healthyIPs, dc.failedIPs, dc.lastUpdated, dc.lastError
}

func (dc *domainChecker) setState(healthyIPs []string, failedIPs map[string]int) {
	dc.lock.Lock()
	defer dc.lock.Unlock()

	dc.healthyIPs = healthyIPs
	dc.failedIPs = failedIPs
	dc.lastUpdated = time.Now()
	dc.lastError = ""
}

func (dc *domainChecker) setError(err string) {
	dc.lock.Lock()
	defer dc.lock.Unlock()

	dc.lastUpdated = time.Now()
	dc.lastError = err
}

// getRecovering returns a copy of the recovery counters
func (dc *domainChecker) getRecovering() map[string]int {
	dc.lock.Lock()
	defer dc.lock.Unlock()

	res := make(map[string]int, len(dc.recovering))
	maps.Copy(res, dc.recovering)
	return res
}

func (dc *domainChecker) setRecovering(r map[string]int) {
	dc.lock.Lock()
	defer dc.lock.Unlock()

	dc.recovering = r
}

// swapAllDown sets the all-down flag and returns the previous value
func (dc *domainChecker) swapAllDown(v bool) bool {
	dc.lock.Lock()
	defer dc.lock.Unlock()

	prev := dc.allDown
	dc.allDown = v
	return prev
}

// swapNotifiedError sets the last notified error and returns the previous value
func (dc *domainChecker) swapNotifiedError(v string) string {
	dc.lock.Lock()
	defer dc.lock.Unlock()

	prev := dc.notifiedError
	dc.notifiedError = v
	return prev
}

// getEndpoints returns a copy of the endpoints by target
func (dc *domainChecker) getEndpoints() map[string]*config.ConfigEndpoint {
	dc.lock.Lock()
	defer dc.lock.Unlock()

	return maps.Clone(dc.endpoints)
}

// noteTargets records the endpoints that look up their IP address under the address they found
// Endpoints whose address is not in the keep list (the current and previous targets) are forgotten
func (dc *domainChecker) noteTargets(results []checker.Result, keep ...[]string) {
	dc.lock.Lock()
	defer dc.lock.Unlock()

	if dc.endpoints == nil {
		dc.endpoints = make(map[string]*config.ConfigEndpoint)
	}

	keepSet := make(map[string]struct{})
	for _, list := range keep {
		for _, t := range list {
			keepSet[t] = struct{}{}
		}
	}

	for _, r := range results {
		if r.Endpoint.Dynamic() && r.Target != "" {
			dc.endpoints[r.Target] = r.Endpoint
			keepSet[r.Target] = struct{}{}
		}
	}

	for target, ep := range dc.endpoints {
		_, ok := keepSet[target]
		if !ok && ep.Dynamic() {
			delete(dc.endpoints, target)
		}
	}
}

// dedupeResults keeps one result per target
// Endpoints that look up their IP address can find the same one (for example, a primary and a fallback lookup service), and a target must be published only once
// If one of the results is healthy it wins; otherwise the first is kept
func dedupeResults(results []checker.Result) []checker.Result {
	index := make(map[string]int, len(results))
	out := make([]checker.Result, 0, len(results))
	for _, r := range results {
		target := r.Target
		if target == "" {
			target = r.Endpoint.Target()
		}
		// Results without a target are kept as they are, since they can't clash with anything
		if target == "" {
			out = append(out, r)
			continue
		}

		i, ok := index[target]
		switch {
		case !ok:
			index[target] = len(out)
			out = append(out, r)
		case r.Healthy && !out[i].Healthy:
			out[i] = r
		}
	}
	return out
}
