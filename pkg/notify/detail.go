package notify

// This file is a fork extension: the detailed messages, enabled with notifications.detail
// It restores what the fork sent before the typed notifications: the status emoji, the priority tiers, the standby endpoints and the error of each unhealthy endpoint
// Upstream code calls it from three places only: Event.Detail, Notifier.message and Notify
// To drop it, delete this file and those call sites, see FORK.md

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
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
	// The health checker removes URLs from it
	Error string
	// Name of the endpoint when it was set in the configuration, used in titles and in the list of endpoints
	// Empty when the name is only the default, which can be the URL of the health check
	Label string
}

// message returns the message for an event, with the details if they are enabled
func (n *Notifier) message(ev Event) message {
	m := newMessage(ev)
	if !n.detail {
		return m
	}
	return withDetail(m, ev)
}

// tidyDetail puts the error of each endpoint on one line and cuts it
// It returns a copy, because the same detail can be shared by events that are still being delivered
func tidyDetail(d Detail) Detail {
	if len(d.Endpoints) == 0 {
		return d
	}

	clean := maps.Clone(d.Endpoints)
	for addr, ep := range clean {
		// Joined errors, like those of the IP lookups, span several lines
		msg := strings.ReplaceAll(ep.Error, "\n", "; ")
		ep.Error = truncateRunes(msg, maxEndpointErrorLen)
		clean[addr] = ep
	}
	return Detail{
		Endpoints: clean,
	}
}

// withDetail replaces the title with one that names the endpoints and starts with the status emoji, and replaces the body with the detailed one
// The ntfy title keeps no emoji, because ntfy shows the status tag before the title
func withDetail(m message, ev Event) message {
	status := ev.status()
	m.ntfyTitle = detailTitle(ev, m.title)
	m.title = statusEmoji(status) + " " + m.ntfyTitle
	m.body = detailBody(ev, status)
	return m
}

// detailTitle returns the title of an event, with the names of the endpoints instead of their addresses
// The other events keep the title of upstream, passed as fallback
func detailTitle(ev Event, fallback string) string {
	switch ev.Type {
	case config.EventDNSUpdated:
		return fmt.Sprintf("%s now points to %s", ev.Domain, strings.Join(ev.Detail.names(ev.Published), ", "))
	case config.EventDNSUpdateFailed:
		if len(ev.Published) == 0 {
			return fallback
		}
		return fmt.Sprintf("DNS update failed for %s (wanted %s)", ev.Domain, strings.Join(ev.Detail.names(ev.Published), ", "))
	case config.EventAllUnhealthy:
		// Nothing is published, and every endpoint is down
		return fallback
	default:
		return fallback
	}
}

// names returns what to call the endpoints with the given addresses: their label, or the address for those without one
// The addresses are sorted first, so the order is stable, and endpoints that share a name are listed once
func (d Detail) names(addrs []string) []string {
	sorted := slices.Clone(addrs)
	slices.Sort(sorted)
	res := make([]string, 0, len(sorted))
	for _, addr := range sorted {
		name := cmp.Or(safeLabel(d.Endpoints[addr].Label), addr)
		if !slices.Contains(res, name) {
			res = append(res, name)
		}
	}
	return res
}

// safeLabel returns the label, or an empty string if it looks like a URL or has credentials, so it can never put one in a title
func safeLabel(label string) string {
	if strings.Contains(label, "://") || strings.Contains(label, "@") || strings.Contains(label, redacted) {
		return ""
	}
	return label
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
	label := safeLabel(d.Label)
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

	// Endpoints without a label are shown by address, because their default name can be the URL of the health check
	switch {
	case label != "" && showPriority:
		fmt.Fprintf(b, "  %s (%s, priority %d): %s\n", label, ep.IP, d.Priority, state)
	case label != "":
		fmt.Fprintf(b, "  %s (%s): %s\n", label, ep.IP, state)
	case showPriority:
		fmt.Fprintf(b, "  %s (priority %d): %s\n", ep.IP, d.Priority, state)
	default:
		fmt.Fprintf(b, "  %s: %s\n", ep.IP, state)
	}
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
