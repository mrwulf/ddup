package healthcheck

// This file is a fork extension: the details of the endpoints for the detailed notifications, see pkg/notify/detail.go and FORK.md

import (
	"errors"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/italypaleale/ddup/pkg/healthcheck/checker"
	"github.com/italypaleale/ddup/pkg/notify"
)

// Credentials in a URL, as in scheme://user:password@host
var urlUserinfoRegexp = regexp.MustCompile(`(?i)\b([a-z][a-z0-9+.-]*://)[^/\s@"]+@`)

// newNotifyDetail returns the details of the endpoints after a check
// The priorities are by address, and the published addresses are those selected for DNS
func newNotifyDetail(results []checker.Result, priorities map[string]int, published []string) notify.Detail {
	endpoints := make(map[string]notify.EndpointDetail, len(results))
	for _, result := range results {
		ip := resultIP(result)
		endpoints[ip] = notify.EndpointDetail{
			Priority: priorities[ip],
			Active:   ip != "" && slices.Contains(published, ip),
			Error:    checkErrorText(result.Error),
		}
	}
	return notify.Detail{
		Endpoints: endpoints,
	}
}

// notifyDetail returns the details of the endpoints of a domain after its latest check
func (hc *HealthChecker) notifyDetail(domain string) notify.Detail {
	dc, ok := hc.domainCheckers[domain]
	if !ok {
		return notify.Detail{}
	}
	return dc.notifyDetail
}

// checkErrorText returns the error of a health check without the URL of the endpoint
// The URL can contain a token in its path or query string, and net/http includes it in its errors; only the cause is kept
// Credentials in any other URL are removed too
func checkErrorText(err error) string {
	if err == nil {
		return ""
	}

	msg := err.Error()
	urlErr, ok := errors.AsType[*url.Error](err)
	if ok && urlErr.Err != nil {
		msg = strings.ReplaceAll(msg, urlErr.Error(), urlErr.Err.Error())
	}
	return urlUserinfoRegexp.ReplaceAllString(msg, "${1}[redacted]@")
}
