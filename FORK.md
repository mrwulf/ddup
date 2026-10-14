# What this fork adds

This fork of [ItalyPaleAle/ddup](https://github.com/ItalyPaleAle/ddup) carries features that are not upstream, or not yet. Upstream `main` is `23ac0bb`; the failover priorities, the segmented health bar, the tab icon with the overall status, the Check now button and the health check options are upstream's and are not listed here.
Everything shown below is sample data: `example.com` names, documentation IP addresses (`192.0.2.0/24`, `198.51.100.0/24`, `203.0.113.0/24`, `2001:db8::/32`), and a made-up tunnel ID.
[`UPSTREAM.md`](UPSTREAM.md) tracks each feature below against its upstream PR or decision.
[`config.sample.yaml`](config.sample.yaml) documents every option.

## Dashboard

![The dashboard in the dark theme, with the red cross icon next to the title and the summary bar under the search bar: failover tiers, CNAME and proxied targets, dynamic DNS, a warning, and an error](screenshots/dashboard-dark.webp)

Each endpoint row shows what ddup publishes and why. Upstream shows the endpoints of a domain by address; the rows here are fork-only for now (upstream issue [#27](https://github.com/ItalyPaleAle/ddup/issues/27), after the features below):

- **Priority and standby.** The row shows the endpoint's priority, and whether it is published in DNS or healthy on standby (see `fast-lb` and `ingress-lb`). The selection of the lowest priority is upstream's; the display and the published set in the status API are not.
- **CNAME and proxied targets.** An endpoint can publish a CNAME, optionally proxied by Cloudflare, for example a Cloudflare Tunnel as the last resort (`app` has failed over to its tunnel). Upstream PR [#38](https://github.com/ItalyPaleAle/ddup/pull/38) adds `cname` and `providerOptions.cloudflare.proxied`; this fork uses a `proxied` key on the endpoint.
- **Dynamic DNS.** A record can follow the public IP address of the network ddup runs in (`home`), with `ipLookup` on the endpoint. Upstream PR [#39](https://github.com/ItalyPaleAle/ddup/pull/39) does it with `ipLookupFamily` and `ipLookupURLs`.
- **Failed check counts and domain errors.** The warning badge on an endpoint is its number of consecutive failed checks. A domain that can't be updated shows the provider error (`status`).
- **Legend.** The footer explains every icon.
- **Version footer.** The last line shows the version of the build, fetched from `GET /api/info`. Upstream PR [#35](https://github.com/ItalyPaleAle/ddup/pull/35) writes it into the page instead.
- **Since.** Hovering the status icon of an endpoint shows since when it has been healthy or failing, and its last failure, from timestamps kept in memory (not stored; "since ddup started" if it was never seen changing). Upstream PR [#37](https://github.com/ItalyPaleAle/ddup/pull/37) has a popover instead of a tooltip.

### Theme

The last button in the header cycles the theme between system, light and dark, and the choice is remembered in the browser. Upstream declined it ([#20](https://github.com/ItalyPaleAle/ddup/pull/20)): it stays in the fork on purpose.

![The auto-refresh menu open](screenshots/auto-refresh-menu.webp)

### Light theme

![The dashboard in the light theme](screenshots/dashboard-light.webp)

### Small screens

<img src="screenshots/dashboard-mobile-dark.webp" alt="The dashboard on a phone-sized screen" width="320">

## Not visible in the dashboard

- **Leader election.** With `leaderElection.enabled`, several replicas run on Kubernetes: all of them check health, but only the one holding a Lease updates DNS and sends webhooks. Standby replicas forward API requests to the leader, so every dashboard shows the leader's view. Upstream declined it ([#30](https://github.com/ItalyPaleAle/ddup/issues/30)): fork-only on purpose.
- **Webhooks.** Called on `dns_updated`, `dns_update_failed` and `all_unhealthy`, with templated headers and body, retries, and the domain's health status for templates (for example to set an ntfy tag). The sample config shows ntfy and an HTTP email API. Upstream PR [#40](https://github.com/ItalyPaleAle/ddup/pull/40) replaces them with typed notifications.
- **Webhook `vars`.** Values for the templates of a webhook (`{{ .Vars.token }}`), loaded with upstream's `!file` and `!envfile`, for secrets that go inside a header or body template. Not needed with typed notifications.
- **Typed DNS targets.** Every DNS provider takes `[]Target`; providers that only publish plain A and AAAA records (Azure, OVH, AWS Route53, Google Cloud DNS) reject CNAME and proxied targets.

## Regenerating the screenshots

The images come from the dashboard's `dashboarddev` mode, which serves static data instead of running health checks (`cmd/ddup/main_dashboarddev.go`).
To regenerate them, replace the mock data with the example values listed at the top of this page, build with `-tags dashboarddev`, and capture the page with a headless browser. Don't commit that data change.
`tab-states.webp` is a capture of a real, non-headless Chrome window pointed at that server, with neutral sample pages in the other tabs; headless Chrome has no tab strip.
Never take screenshots from a real deployment: they would show real names and addresses.
