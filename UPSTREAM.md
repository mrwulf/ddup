# Upstream tracking

This file is specific to this fork: it tracks what has been, and still has to be, submitted to
[ItalyPaleAle/ddup](https://github.com/ItalyPaleAle/ddup). It must not be part of any upstream PR.

Last reviewed: 2026-10-14 (dates are UTC). Upstream `main` is `23ac0bb` (AWS Route53 and Google Cloud DNS), and this fork's `main` records it as merged.
go-kit (the maintainer's library, [italypaleale/go-kit](https://github.com/italypaleale/go-kit)) is at v1.5.0; this fork's `go.mod` still uses 1.4.1 until [#40](https://github.com/ItalyPaleAle/ddup/pull/40) merges.

## Syncing with upstream

Upstream squashes PRs, so histories differ. This fork's `main` records upstream as merged with `git merge -s ours`, which brings in no content.
Because of that, **the content has to be ported by hand, and checked file by file**: after each sync run `git diff upstream/main origin/main --stat` and make sure every difference is a fork-only feature (see below).
An earlier sync missed the maintainer's changes to `pkg/server/static.go`, `dashboard/scripts/build-favicons.ts` and the `Makefile`, which came with the squashed #17, #18 and #34.

1. `git fetch upstream`, then (as a separate command) `git fetch origin`.
2. `git log --oneline origin/main..upstream/main` lists what is missing. The maintainer also pushes commits onto our PR branches (for example "💄" style commits): look at them before porting.
3. Port each commit by hand (`git cherry-pick -n` and fix the conflicts) to the fork's types. Code that touches DNS providers must use `[]Target` and `ipsFromTargets`; providers that only publish plain A/AAAA records reject CNAME and proxied targets with `ErrUnsupportedTarget`.
4. Record it: `git merge -s ours upstream/main -m "chore: record upstream/main as merged (<sha>)"`, after the content is ported.

Always branch PRs from `upstream/main` and apply only the listed changes; never open a PR from this fork's `main`.

### Rules from the maintainer

- **Commits must be signed.** Signing is set up on this machine (SSH key registered on GitHub as a signing key; `commit.gpgsign` and `tag.gpgsign` are on). Commits before `v0.6.0-fork.17` and the earlier tags are unsigned.
- **One focused PR per feature, not stacked. Open an issue first for a new feature** and wait for agreement. Bug fixes can go straight to a PR.
- Read [`AGENTS.md`](https://github.com/ItalyPaleAle/ddup/blob/main/AGENTS.md) before every PR. In short:
  - Comments: one sentence per line, never wrapped, no trailing period on single-line comments.
  - Go: never declare variables inside `if` conditions; struct literals with one field per line; named results for `UpdateRecords`-style functions; `require.ErrorContains` instead of `assert.Contains(err.Error(), ...)`; run `gofmt`.
  - Run Go tests with `-tags unit`, and never put the `unit` tag in a `_test.go` file.
  - TS/JS: braces around every control flow body; pnpm, not npm; every enabled `<button>` needs `cursor: pointer`.
  - `AGENTS.md` still says to run `make gen-config`, but the maintainer removed that target (there is no `tools/gen-config`): there is nothing to run.
- Style the maintainer enforces in reviews: explicit over inferred (the config says what a record is, the code does not guess); hand-rolled over a new dependency (the history popover in [#37](https://github.com/ItalyPaleAle/ddup/pull/37) has no UI library); no AI-flavored over-defensiveness (no defensive checks or fallbacks for cases that can't happen); no provider-specific keys in the generic part of the config; keep new config options minimal.
- Show how a UI change looks (screenshots) and offer design options for visual choices.
- Upstream will have a `DEVELOPMENT.md` once [#36](https://github.com/ItalyPaleAle/ddup/pull/36) merges; until then use the plain commands in the checklist at the end.
- golangci-lint is pinned in `.golangci-lint-version` (with the leading `v`); install that release binary, because package manager builds refuse this module's Go version.
- The maintainer plans a release once the open PRs he agreed to are in.

## Status of our upstream PRs

| PR | What | State | Branch | Fork equivalent |
| :- | :--- | :---- | :----- | :-------------- |
| [#6](https://github.com/ItalyPaleAle/ddup/pull/6) | Health check `method`, `expectStatus`, `recoverAfter` | **Merged** 2026-10-03 (`1499afb`) | `pr/1-health-check-options` | Same code in `pkg/healthcheck` |
| [#16](https://github.com/ItalyPaleAle/ddup/pull/16) | Document the Cloudflare token permissions | **Merged** 2026-10-03 | `pr/11-docs-cf-token` | README |
| [#9](https://github.com/ItalyPaleAle/ddup/pull/9) | "Check now" button and `POST /api/check` | **Merged** 2026-10-03 | `pr/4-check-now` | Same |
| [#15](https://github.com/ItalyPaleAle/ddup/pull/15) | `UpdateRecords` returns an `UpdateResult` (`Changed`, `Previous`) | **Merged** 2026-10-03 | `pr/10-update-result` | Same |
| [#17](https://github.com/ItalyPaleAle/ddup/pull/17) | Stop caching the dashboard page for 24 hours | **Merged** 2026-10-04 (`c37b5cf`) | `pr/12-cache-headers` | `pkg/server/static.go` is upstream's |
| [#21](https://github.com/ItalyPaleAle/ddup/pull/21) | A new connection for every health check | **Merged** 2026-10-05 (`c877cd3`) | `pr/16-no-keepalive` | Same |
| [#19](https://github.com/ItalyPaleAle/ddup/pull/19) | Domain health counts as a segmented bar | **Merged** 2026-10-13 (`bf4f058`) | `pr/14-summary-pills` | Same tree |
| [#18](https://github.com/ItalyPaleAle/ddup/pull/18) | Overall status in the tab icon (shapes, pre-rendered PNGs) | **Merged** 2026-10-13 (`7eafb33`), then the maintainer's e2e tests | `pr/13-tab-icon-status` | `dashboard/scripts/build-favicons.ts` and the icons are upstream's |
| [#34](https://github.com/ItalyPaleAle/ddup/pull/34) | Failover priorities (issue #23) | **Merged** 2026-10-13 (`d9557f9`), approved without changes | `feat/priorities` | `pkg/healthcheck/priorities.go` is upstream's; `tiers.go` (`selectPublication`) calls `selectLowestPriority` |
| go-kit [#10](https://github.com/italypaleale/go-kit/pull/10), [#11](https://github.com/italypaleale/go-kit/pull/11), [#12](https://github.com/italypaleale/go-kit/pull/12) | Resend emailer; webhook timeout, retry, private destination and redirect options; ntfy format | **Merged** 2026-10-13 and 2026-10-14, released in go-kit v1.5.0 | | Used by [#40](https://github.com/ItalyPaleAle/ddup/pull/40) |
| [#35](https://github.com/ItalyPaleAle/ddup/pull/35) | Version footer (issue #28): build info written into `index.html` by the server, reload on upgrade | **Open**, changes requested (2026-10-13); the follow-up commits answer the review (server-side injection, e2e tests), no new review yet | `feat/version-footer` | `GET /api/info` and a footer fetched at runtime, the first version of this PR |
| [#36](https://github.com/ItalyPaleAle/ddup/pull/36) | `DEVELOPMENT.md` and Makefile targets (issue #32) | **Open**, no review yet | `docs/development` | None |
| [#37](https://github.com/ItalyPaleAle/ddup/pull/37) | Endpoint history popover: since when healthy or failing (issue #31) | **Open**, no review yet | `feat/endpoint-history` | The "since" tooltip |
| [#38](https://github.com/ItalyPaleAle/ddup/pull/38) | `cname` endpoints and `providerOptions.cloudflare.proxied` (issue #24) | **Open**, no review yet; verified against the real Cloudflare API | `feat/cname-proxied` | `cname` and `proxied` on the endpoint, typed targets |
| [#39](https://github.com/ItalyPaleAle/ddup/pull/39) | Public IP lookup over HTTPS, `ipLookupFamily`, `ipLookupURLs`, no built-in services (issue #25); verified against the real Cloudflare API | **Open**, no review yet | `feat/ip-lookup-https` | Per-endpoint `ipLookup: {urls, family, pattern}` |
| [#40](https://github.com/ItalyPaleAle/ddup/pull/40) | Notifications for ntfy, webhooks and email, on go-kit v1.5.0 (issue #26) | **Open**, no review yet | `feat/notifications` | Templated `webhooks` with `vars` |
| [#20](https://github.com/ItalyPaleAle/ddup/pull/20) | Light/dark/system theme switcher | **Closed** 2026-10-04 by the maintainer: causes theme flicker, respecting the browser preference is right | `pr/15-theme-switcher` | Kept in the fork, on purpose |
| [#8](https://github.com/ItalyPaleAle/ddup/pull/8) | `!env` / `!file` secret references | **Closed** 2026-10-03; replaced by upstream's own `!file` and `!envfile` (issue #22, go-kit 1.4.0) | `pr/3-secret-refs` | Dropped; the fork uses upstream's tags |
| #5, #7, #10 to #14 | The first stacked PRs | **Closed** by us to keep reviews small | `chain/*` | Replaced by the PRs above |

## Status of our upstream issues

| Issue | What | State | What it means for the fork |
| :---- | :--- | :---- | :------------------------- |
| [#22](https://github.com/ItalyPaleAle/ddup/issues/22) | Secrets in the config | Closed, completed 2026-10-12 | `!file` and `!envfile` are used everywhere; `SecretString` is gone |
| [#23](https://github.com/ItalyPaleAle/ddup/issues/23) | Failover priorities | Open; "good idea"; implemented by [#34](https://github.com/ItalyPaleAle/ddup/pull/34) | Done, the issue is left for the maintainer to close |
| [#24](https://github.com/ItalyPaleAle/ddup/issues/24) | Cloudflare proxied records and CNAME targets | Open; design agreed: `ip` and `cname` stay, `providerOptions.cloudflare.proxied` | [#38](https://github.com/ItalyPaleAle/ddup/pull/38). The `type`/`target` idea and a `cnameProxied` key were dropped |
| [#25](https://github.com/ItalyPaleAle/ddup/issues/25) | Dynamic DNS | Open; the maintainer first wanted DNS-based lookups, then accepted HTTPS (starting with Akamai and ipify; icanhazip is not liked) | [#39](https://github.com/ItalyPaleAle/ddup/pull/39) |
| [#26](https://github.com/ItalyPaleAle/ddup/issues/26) | Webhooks | Open; the maintainer wants it in go-kit with typed ntfy and the emailer, not templates | go-kit PRs merged; [#40](https://github.com/ItalyPaleAle/ddup/pull/40) |
| [#27](https://github.com/ItalyPaleAle/ddup/issues/27) | Dashboard: what is published and why, per endpoint | Open; "after the dependencies are done" | No PR yet: the endpoint rows, chips and legend stay fork-only until #38 to #40 are in |
| [#28](https://github.com/ItalyPaleAle/ddup/issues/28) | Version footer | Open; "Sure!" | [#35](https://github.com/ItalyPaleAle/ddup/pull/35) |
| [#29](https://github.com/ItalyPaleAle/ddup/issues/29) | Tracking issue: what the fork needs and how it will be contributed | Open | Priorities, CNAME/proxied and dynamic IP are requirements for this deployment; if upstream declines them, the fork continues alone |
| [#30](https://github.com/ItalyPaleAle/ddup/issues/30) | Leader election | Closed, not planned 2026-10-13: too much to maintain, ddup is not in the critical path, a standby does not replicate state | Fork-only, don't resubmit |
| [#31](https://github.com/ItalyPaleAle/ddup/issues/31) | Since when an endpoint has been healthy or failing | Open; liked; asked for a clickable popover and design options | [#37](https://github.com/ItalyPaleAle/ddup/pull/37) |
| [#32](https://github.com/ItalyPaleAle/ddup/issues/32) | `mise.toml` for the development tools | Open (kept to remember); the maintainer wants a `DEVELOPMENT.md` and Makefile targets instead of mise, and removed `make gen-config` | [#36](https://github.com/ItalyPaleAle/ddup/pull/36). `feat/mise` is not needed |
| [#33](https://github.com/ItalyPaleAle/ddup/issues/33) | Webhook `vars` | Open; made obsolete by the typed notifications (no templates upstream) | The maintainer can close it; fork-only until [#40](https://github.com/ItalyPaleAle/ddup/pull/40) |

## What the fork has that upstream does not

| Feature | Where | Status |
| :------ | :---- | :----- |
| Leader election and standby request forwarding | `pkg/leader`, `pkg/server/leaderproxy.go`, `leaderElection` | Declined (#30). Fork-only on purpose; every other feature must work with it off |
| `cname` endpoints and `proxied` on the endpoint, typed DNS targets (`dns.Target`) | `pkg/dns`, `pkg/config` | Open upstream PR [#38](https://github.com/ItalyPaleAle/ddup/pull/38) (different config key) |
| Dynamic IP: per-endpoint `ipLookup: {urls, family, pattern}` | `pkg/healthcheck/checker/iplookup.go` | Open upstream PR [#39](https://github.com/ItalyPaleAle/ddup/pull/39) (different config) |
| Templated `webhooks` with `vars`, retries, status tags | `pkg/notify` | Open upstream PR [#40](https://github.com/ItalyPaleAle/ddup/pull/40) replaces them with typed `notifications` |
| "Since" timestamps in a tooltip (`since`, `sinceStart`, `lastFailure` in the status API) | `pkg/healthcheck/status.go`, dashboard | Open upstream PR [#37](https://github.com/ItalyPaleAle/ddup/pull/37) (popover) |
| Version footer from `GET /api/info` | `pkg/server`, dashboard | Open upstream PR [#35](https://github.com/ItalyPaleAle/ddup/pull/35) (different mechanism) |
| Endpoint rows with names, priority, standby and failed-check chips, a legend, and the published set and tier in the status API and webhooks | dashboard, `pkg/healthcheck/tiers.go`, `status.go` | Issue [#27](https://github.com/ItalyPaleAle/ddup/issues/27), after #38 to #40. Not a PR yet |
| Theme switcher (system, light, dark) | `dashboard/src/lib/theme.ts` | Declined (#20). Fork-only on purpose |
| `FORK.md`, `screenshots/`, this file, the fork's README sections | | Fork-only on purpose; keep them out of upstream PRs |
| Release tags `v0.6.0-fork.N` and the publish workflow for the image | `.github/workflows` | Fork-only on purpose |

The providers are now the same as upstream's (Azure, Cloudflare, OVH, AWS Route53, Google Cloud DNS), with one difference: every provider takes `[]Target`, and the ones that only publish A/AAAA records reject CNAME and proxied targets.

## Adoption plan

When an upstream PR merges: port its content by hand (the maintainer may change it before merging, so compare with the merged commit, not with our branch), delete the fork's own code for it, migrate the deployed config in the same deploy (upstream rejects unknown keys, so an old key is a startup error), then record the merge with `-s ours`.
The probable order is below.

### 1. #36 `DEVELOPMENT.md` and Makefile targets

Adds `DEVELOPMENT.md`, the Makefile targets `dashboard`, `build`, `run-dev`, `lint-dashboard`, `test-e2e` and `check`, `/.bin/` in `.gitignore`, and a README link.
The fork's `Makefile` already equals upstream's, so it applies cleanly. Keep the fork-only text in the README, and add the link at the end. Then the checklist below can point to `make check`.

### 2. #35 Version footer

- Delete `GET /api/info`, `buildInfoResponse`, `TestInfoEndpoint` and the dashboard code that fetches it.
- Take the server-side injection (a JSON script element `ddup-build` in `index.html`), the `X-Ddup-Build` header on API responses, and the reload on upgrade (at most 3 tries per server build, 30 seconds apart; no reload without `sessionStorage`).
- The footer text changes from `ddup <version> · built <date> · <commit>` to `ddup <version> · <short commit>`. The build date is no longer shown.
- The fork's version string (`0.6.0-fork.N`) still comes from `pkg/buildinfo`, so it appears as before. The fork's e2e test for the footer is replaced by the PR's `build.spec.ts`.

### 3. #37 History popover

- The status API fields stay the same (`since`, `sinceStart`, `lastFailure`, all optional); the Go side should need only a comparison with the PR.
- Replace the tooltip with the `EndpointHistory` component: a "History" button on the "N/N endpoints healthy" line opens a dialog listing every endpoint of the domain. Replace the fork's e2e test `says since when an endpoint is healthy or failing`, which reads the tooltip.

### 4. #38 `cname` and `providerOptions.cloudflare.proxied`

Config: `proxied: true` on the endpoint becomes
```yaml
providerOptions:
  cloudflare:
    proxied: true
```
`ip` and `cname` stay. The rule "endpoints with the same priority use the same `proxied`" stays, with a new message; `ConfigEndpoint.Proxied` (field) becomes a method.
The fork's check "cname and proxied are only supported by Cloudflare" goes away: providers declare `Capabilities()` (`Type`, `CNAME`, `ProviderOptions`) and `dns.ValidateConfig` checks the config at startup. **Every provider needs `Capabilities()`, including AWS Route53 and Google Cloud DNS, which only support plain A/AAAA records.** The PR branch has them.
Code:
- `Target.RecordType()` (guessing A, AAAA or CNAME from the value) goes; `Target` has an explicit `Type`, built with `dns.IPTarget(ip, proxied)` and `dns.CNAMETarget(host, proxied)`. `IPTargets(...)` and `ipsFromTargets` become `IPTarget` per IP and `plainIPsFromTargets`.
- The Cloudflare provider is upstream's (the logic matches the fork's: match by type and value, TTL 1 for proxied, overwrite in place between CNAME and A/AAAA, delete first as a fallback).
- The status API gets `type` and `proxied` per endpoint (the fork has more fields than the PR; keep them). The dashboard chips for CNAME and proxied can read them instead of deriving them.
- Keep the fork's handling of dynamic-IP endpoints (target known only after a lookup) in the duplicate-target check.

### 5. #39 Dynamic IP over HTTPS

- Config: the fork's per-endpoint `ipLookup: {urls, family, pattern}` becomes `ipLookupFamily: 4` plus `ipLookupURLs: [{url: ..., pattern: ...}]` on the endpoint, or one list at the top level of the config shared by endpoints. Nothing is built in: every service must be listed. `pattern` moves to the entry it applies to (repeat it if it applied to all).
- Behavior changes: all the services are asked at once and the first valid answer wins (a warning is logged if two disagree), instead of trying them in order; URLs must be `https`; redirects are not followed; the connection is forced to the IP version of `ipLookupFamily`; proxy environment variables are ignored for lookups; the default endpoint name is `IPv4 lookup` or `IPv6 lookup`; there is no cache shared across domains.
- Delete `pkg/healthcheck/checker/iplookup.go` and its test, `ConfigIPLookup`, `config.RedactURL` (becomes `iplookup.RedactURL`), and the sequential loop. Keep the fork's `Endpoint.Target()`, `dedupeResults`, `noteTargets` and `selectPublication`, which also cover `cname` and `proxied`; drop upstream's `dynamicPriorities`, which the fork does not need.
- `validateAddress` gets `cname` as a third alternative ("one of ip, cname and ipLookupFamily").
- Check the lookup client before switching: the fork follows redirects and honors proxy variables.

### 6. #40 Notifications

The fork's `webhooks` with `vars` become typed `notifications` (`ntfy`, `webhooks`, `email`), each entry with a unique `name`:
- ntfy: `url`, `token` (the bare token; ddup adds `Bearer `), `timeout`, `maxAttempts`, `retryInterval`. **A private in-cluster URL needs `allowPrivateDestinations: true`**, otherwise deliveries fail with a logged error.
- Email goes through go-kit's emailer: a connection string (SMTP, with `tls=none` for an in-cluster relay, or `resend://<key>?fromAddress=...`) and `to`, one email per address. Secrets are typed fields read with `!file`, so `vars` goes away.
- Lost: templates, custom headers and method, the JSON catch-all receiver, the Cloudflare Email Service example, email retries (the emailer sends once), the tier line and `standby` in the text, endpoint names in the title (it lists the addresses), the status emoji in the text, the exponential backoff (fixed `retryInterval`; use `maxAttempts: 5` and `retryInterval: 2s` for a similar window).
- Changed: ntfy priority is not configurable (urgent when nothing is healthy, high when the DNS update failed, the server default otherwise); events are queued per destination (64, new ones dropped when full); error text is cleaned of secrets.
- Kept in the fork: the `isLeader` gate around the call that queues the event.
- `go.mod` goes to go-kit v1.5.0.

## Branches on the fork (listing only, nothing is deleted)

| Branches | What | State |
| :------- | :--- | :---- |
| `feat/version-footer` (#35), `docs/development` (#36), `feat/endpoint-history` (#37), `feat/cname-proxied` (#38), `feat/ip-lookup-https` (#39), `feat/notifications` (#40) | Heads of the open upstream PRs | Keep until the PR is merged or closed |
| `pr/1-health-check-options` (#6), `pr/3-secret-refs` (#8), `pr/4-check-now` (#9), `pr/10-update-result` (#15), `pr/11-docs-cf-token` (#16), `pr/12-cache-headers` (#17), `pr/13-tab-icon-status` (#18), `pr/14-summary-pills` (#19), `pr/15-theme-switcher` (#20), `pr/16-no-keepalive` (#21), `feat/priorities` (#34) | Heads of merged or closed upstream PRs | Obsolete |
| `chain/1-webhooks` to `chain/5-iplookup` | The first stacked PR branches (closed) | Obsolete |
| `feat/ip-lookup-https-v2`, `feat/version-footer-v2`, `feat/version-footer-v3` (same commit as `feat/version-footer`), `feat/tab-icon-v2`, `feat/health-semantics-and-webhooks`, `chore/retire-secretstring` | Earlier drafts of the PRs above | Obsolete |
| `feat/mise` | `mise.toml` for [#32](https://github.com/ItalyPaleAle/ddup/issues/32) | Not needed: the maintainer chose `DEVELOPMENT.md` (#36) |
| `pr-assets` | Images used in PR and issue descriptions | Keep while those links are in use |
| `main` | The fork's line; what is deployed | |
| `chore/*`, `docs/*` other than `docs/development`, `ci/publish-image`, `fix/lint-ci`, `feat/leader-election`, `feat/endpoint-since`, `feat/summary-bar`, `feat/tab-icon-*`, `feat/webhook-vars`, `feat/dashboard-endpoints-ui`, `feat/ip-lookup`, `feat/tiers-and-typed-targets` | Work already merged into `main` | Obsolete |

The local `main` in the main checkout is 48 commits behind `origin/main`; update it before using it.

## Releases

The image is `ghcr.io/mrwulf/ddup`, built by `.github/workflows/build-and-publish.yaml` when a `v*` tag is pushed (and for `main` as `edge`).
The tags are `v0.6.0-fork.N`. The latest is `v0.6.0-fork.22` (2026-10-12), and it is what is deployed.
`main` is 28 commits ahead of it, with no tag yet: the upstream sync of 2026-10-12 and 2026-10-13 (segmented bar, tab icon shapes, upstream's priority selection, go-kit helpers, e2e tests), and the two providers from the sync after that.

| Tag | Added |
| :-- | :---- |
| `fork.17` | Version footer (the first signed tag; the earlier ones are unsigned) |
| `fork.18` to `fork.20` | Leader election, its defaults, request forwarding |
| `fork.21` | Webhook `vars` |
| `fork.22` | Refreshed screenshots (docs only) |

## Before opening a PR

1. Open an issue first if it is a new feature, and wait for agreement.
2. `git fetch upstream`, then branch from `upstream/main`.
3. Cherry-pick only the commits for one feature, and remove anything fork-only (see above).
4. Run the checks (until `DEVELOPMENT.md` and the Makefile targets of #36 are merged, use the plain commands):
   - `mkdir -p dashboard/dist && touch dashboard/dist/.placeholder` (the Go code embeds `dashboard/dist`; it is gitignored)
   - `go vet -tags unit ./...` and `go vet -tags unit,dashboarddev ./cmd/...`
   - `gofmt -l pkg cmd` lists nothing
   - `go test -race -tags unit ./pkg/...`
   - `golangci-lint run` with the version in `.golangci-lint-version`
   - For dashboard changes, in `dashboard`: `pnpm install --frozen-lockfile`, `pnpm type-check lint format:check build` under Node 24, and `pnpm test:e2e` (needs `pnpm exec playwright install chromium` once), plus screenshots of the change
5. Make sure the commits are signed (`git log --show-signature`), and that the code and the PR text follow `AGENTS.md`.
6. One focused PR, not stacked, with a description that says how it was tested.
