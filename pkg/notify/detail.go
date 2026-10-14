package notify

// This file is a fork extension: the detailed messages, enabled with notifications.detail
// It restores what the fork sent before the typed notifications: the status emoji, the priority tiers, the standby endpoints and the error of each unhealthy endpoint
// Upstream code calls it from three places only: Event.Detail, Notifier.message and Notifier.redactDetail
// To drop it, delete this file and those call sites, see FORK.md

import (
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/italypaleale/ddup/pkg/config"
)

// Maximum length of the error of an endpoint in a message, in runes
const maxEndpointErrorLen = 200

// Detail is what the detailed messages add to an event
// The zero value is valid, and the messages show what they can without it
type Detail struct {
	// Details of the endpoints, by address (the IP field of EndpointState)
	Endpoints map[string]EndpointDetail
}

// EndpointDetail is what the detailed messages show for an endpoint, in addition to EndpointState
type EndpointDetail struct {
	// Priority of the endpoint: only the healthy endpoints with the lowest value are published
	Priority int
	// True if the endpoint is published in DNS, or would be after a failed update
	Active bool
	// Error of the latest check, if it failed
	// The health checker removes URLs from it, and the notifier removes known secrets
	Error string
}

// message returns the message for an event, with the details if they are enabled
func (n *Notifier) message(ev Event) message {
	m := newMessage(ev)
	if !n.detail {
		return m
	}
	return withDetail(m, ev)
}

// redactDetail removes the secrets from the errors of the endpoints
// It returns a copy, because the same detail can be shared by events that are still being delivered
func (n *Notifier) redactDetail(d Detail) Detail {
	if len(d.Endpoints) == 0 {
		return d
	}

	clean := maps.Clone(d.Endpoints)
	for addr, ep := range clean {
		if ep.Error == "" {
			continue
		}
		// Joined errors, like those of the IP lookups, span several lines
		msg := strings.ReplaceAll(n.redact.String(ep.Error), "\n", "; ")
		ep.Error = truncateRunes(msg, maxEndpointErrorLen)
		clean[addr] = ep
	}
	return Detail{
		Endpoints: clean,
	}
}

// withDetail adds the status emoji to the title, and replaces the body with the detailed one
// The ntfy title keeps no emoji, because ntfy shows the status tag before the title
func withDetail(m message, ev Event) message {
	status := ev.status()
	m.ntfyTitle = m.title
	m.title = statusEmoji(status) + " " + m.title
	m.body = detailBody(ev, status)
	return m
}

// statusEmoji returns a green, yellow or red circle for the status of the domain, like the ntfy tags
func statusEmoji(status string) string {
	switch status {
	case StatusHealthy:
		return "🟢"
	case StatusWarning:
		return "🟡"
	default:
		return "🔴"
	}
}

// detailBody is the body of the detailed messages
// It has the same lines as messageBody, with the status emoji, the priority tier when it changed, and the endpoints with their tier, standby state and error
func detailBody(ev Event, status string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Event: %s\nDomain: %s\nTime: %s\nStatus: %s %s\n", ev.Type, ev.Domain, ev.Time.UTC().Format(time.RFC3339), statusEmoji(status), status)

	switch ev.Type {
	case config.EventDNSUpdated:
		writeAddresses(&b, "Now", ev.Published)
		writeAddresses(&b, "Before", ev.Previous)
		writeTierChange(&b, ev)
	case config.EventDNSUpdateFailed:
		writeAddresses(&b, "Wanted", ev.Published)
		writeAddresses(&b, "Before", ev.Previous)
	case config.EventAllUnhealthy:
		writeAddresses(&b, "Unchanged", ev.Previous)
	}

	if ev.Error != "" {
		fmt.Fprintf(&b, "Error: %s\n", ev.Error)
	}

	if len(ev.Endpoints) > 0 {
		// The priority of each endpoint is only worth showing when the domain has more than one tier
		showPriority := ev.Detail.tierCount(ev.Endpoints) > 1
		b.WriteString("\nEndpoints:\n")
		for _, ep := range ev.Endpoints {
			writeEndpoint(&b, ep, ev.Detail.Endpoints[ep.IP], showPriority)
		}
	}

	return b.String()
}

// writeTierChange writes the priority of the published endpoints, when it differs from the one of the previous records
func writeTierChange(b *strings.Builder, ev Event) {
	tier, ok := ev.Detail.tierOf(ev.Published)
	if !ok {
		return
	}
	previous, ok := ev.Detail.tierOf(ev.Previous)
	if !ok || previous == tier {
		return
	}
	fmt.Fprintf(b, "Priority: %d (was %d)\n", tier, previous)
}

func writeEndpoint(b *strings.Builder, ep EndpointState, d EndpointDetail, showPriority bool) {
	state := "healthy"
	switch {
	case !ep.Healthy && d.Error != "":
		state = "UNHEALTHY (" + d.Error + ")"
	case !ep.Healthy:
		state = "UNHEALTHY"
	case showPriority && !d.Active:
		// A healthy endpoint that is not published waits for the published tier to fail
		state = "healthy (standby)"
	}

	if showPriority {
		fmt.Fprintf(b, "  %s (%s, priority %d): %s\n", ep.Name, ep.IP, d.Priority, state)
		return
	}
	fmt.Fprintf(b, "  %s (%s): %s\n", ep.Name, ep.IP, state)
}

// tierCount returns the number of distinct priorities among the endpoints
func (d Detail) tierCount(endpoints []EndpointState) int {
	tiers := make(map[int]struct{}, len(endpoints))
	for _, ep := range endpoints {
		tiers[d.Endpoints[ep.IP].Priority] = struct{}{}
	}
	return len(tiers)
}

// tierOf returns the lowest priority among the endpoints with the given addresses
// It returns false if none of them is a known endpoint
func (d Detail) tierOf(addrs []string) (int, bool) {
	var (
		tier  int
		found bool
	)
	for _, addr := range addrs {
		ep, ok := d.Endpoints[addr]
		if !ok {
			continue
		}
		if !found || ep.Priority < tier {
			tier = ep.Priority
			found = true
		}
	}
	return tier, found
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
