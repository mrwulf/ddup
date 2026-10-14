# What this fork adds

This fork of [ItalyPaleAle/ddup](https://github.com/ItalyPaleAle/ddup) carries features that are not upstream, or not yet.
This branch is upstream `main` (`23ac0bb`) with the open upstream pull requests [#35](https://github.com/ItalyPaleAle/ddup/pull/35) to [#40](https://github.com/ItalyPaleAle/ddup/pull/40) merged, and the fork-only features below on top. What those pull requests add (the version footer, the endpoint history, `cname` and `proxied`, the IP lookup over HTTPS and the notifications) is not listed here.
Everything shown below is sample data: `example.com` names, documentation IP addresses (`192.0.2.0/24`, `198.51.100.0/24`, `203.0.113.0/24`, `2001:db8::/32`), and a made-up tunnel ID.
[`UPSTREAM.md`](UPSTREAM.md) tracks each feature below against its upstream PR or decision.
[`config.sample.yaml`](config.sample.yaml) documents every option.

## Dashboard

![The dashboard in the dark theme, with the red cross icon next to the title and the summary bar under the search bar: failover tiers, CNAME and proxied targets, dynamic DNS, a warning, and an error](screenshots/dashboard-dark.webp)

Each endpoint row shows what ddup publishes and why. Upstream shows the endpoints of a domain by address; the rows here are fork-only for now (upstream issue [#27](https://github.com/ItalyPaleAle/ddup/issues/27), after the features above). The status API carries what the rows need: `name`, `priority` and `active` (published in DNS), next to the upstream fields `type`, `proxied` and the times.

- **Name and address.** The name of the endpoint is the main label, with its address below. An endpoint with no name shows its address. The rows are ordered by priority.
- **Priority and standby.** The row shows the endpoint's priority, and whether it is published in DNS or healthy on standby (see `fast-lb` and `ingress-lb`). The selection of the lowest priority is upstream's; the display and the published set in the status API are not.
- **CNAME and proxied targets.** A chip for a CNAME record and one for a record proxied by Cloudflare, for example a Cloudflare Tunnel as the last resort (`app` has failed over to its tunnel). They read `type` and `proxied` from the status.
- **Failed checks and domain errors.** A warning triangle with the number of consecutive failed checks. A domain that can't be updated shows the provider error (`status`).
- **Legend.** The footer explains every icon, above the version of the build.

Every chip has an icon with its own shape and a text for screen readers and the tooltip, so nothing depends on color alone.
The **History** button on the line that counts the healthy endpoints, the version footer and the reload when the server is upgraded are upstream's.

### Theme

The last button in the header cycles the theme between system, light and dark, and the choice is remembered in the browser. Upstream declined it ([#20](https://github.com/ItalyPaleAle/ddup/pull/20)): it stays in the fork on purpose.

![The auto-refresh menu open](screenshots/auto-refresh-menu.webp)

### Light theme

![The dashboard in the light theme](screenshots/dashboard-light.webp)

### Small screens

<img src="screenshots/dashboard-mobile-dark.webp" alt="The dashboard on a phone-sized screen" width="320">

## Not visible in the dashboard

- **Leader election.** With `leaderElection.enabled`, several replicas run on Kubernetes: all of them check health, but only the one holding a Lease updates DNS and sends notifications. Standby replicas forward API requests to the leader, so every dashboard shows the leader's view, and answer with their own build header so a rolling update does not make the page reload. Upstream declined it ([#30](https://github.com/ItalyPaleAle/ddup/issues/30)): fork-only on purpose. The README describes what happens to notifications when the leader changes.

## Detailed notifications

The notifications of upstream PR [#40](https://github.com/ItalyPaleAle/ddup/pull/40) replaced the fork's webhook templates, and their messages lost what the fork used to send. `notifications.detail` (on by default here, `false` to turn it off) brings it back for ntfy, webhooks and email:

- Endpoint names instead of addresses in the titles of `dns_updated` (`example.com now points to server1, server2`) and `dns_update_failed` (`DNS update failed for example.com (wanted server1)`). An endpoint whose name is only the default (its health-check URL, which can contain a token) is shown by its address in titles and in the body; a name that looks like a URL or has credentials is never used.
- A green, yellow or red circle for the status of the domain in the title and in the `Status` line (ntfy keeps it in the tag only, so it isn't shown twice).
- The error of the latest check of each unhealthy endpoint, without the endpoint's URL and with the configured secrets removed.
- On domains with more than one priority, the priority of each endpoint, `healthy (standby)` for a healthy endpoint that is not published, and `Priority: <new> (was <old>)` when a `dns_updated` changes tier.

The README shows an example under [Detailed messages](README.md#detailed-messages).
Nothing changes in when notifications are sent: only the leader sends them, through the same queues.

**How it is kept apart.** The code is in `pkg/notify/detail.go` and `pkg/healthcheck/notifydetail.go`, with their tests. Upstream files have one-line hooks only: the `Detail` field of `notify.Event`, the `ntfyTitle` of `message`, `Notifier.message` and `Notifier.redactDetail` in `notifier.go`, the `notifyDetail` field of `domainChecker` set in `healthchecker.go` and read in `HealthChecker.notify`, and the `Detail` setting of `ConfigNotifications` with its default in `GetDefaultConfig`. A configuration built in code (as in upstream's tests) has `Detail: false`, so upstream's message tests run unchanged.

**How to drop it.** Delete those two files and their tests, then remove the hooks listed above.
Configurations that set `detail` would then fail to load, because unknown keys are rejected.

**Upstreaming.** A candidate once [#40](https://github.com/ItalyPaleAle/ddup/pull/40) is merged: the names in titles, the errors and the tiers are the most useful part, and upstream may prefer them without a setting. The name logic is self-contained: `EndpointDetail.Label` is filled from `displayName` (the configured name, empty for the default) and `Detail.names` falls back to the address, so upstream could fold `Label` into `EndpointState` directly.

## Regenerating the screenshots

The images come from the dashboard's `dashboarddev` mode, which serves static data instead of running health checks (`cmd/ddup/main_dashboarddev.go`, which uses only example names and documentation addresses).
To regenerate them, build with `-tags dashboarddev` and capture the page with a headless browser.
`tab-states.webp` is a capture of a real, non-headless Chrome window pointed at that server, with neutral sample pages in the other tabs; headless Chrome has no tab strip.
Never take screenshots from a real deployment: they would show real names and addresses.
