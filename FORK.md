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

## Regenerating the screenshots

The images come from the dashboard's `dashboarddev` mode, which serves static data instead of running health checks (`cmd/ddup/main_dashboarddev.go`, which uses only example names and documentation addresses).
To regenerate them, build with `-tags dashboarddev` and capture the page with a headless browser.
`tab-states.webp` is a capture of a real, non-headless Chrome window pointed at that server, with neutral sample pages in the other tabs; headless Chrome has no tab strip.
Never take screenshots from a real deployment: they would show real names and addresses.
