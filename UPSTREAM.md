# Upstream tracking

This file is specific to this fork: it tracks what has been, and still has to be, submitted to
[ItalyPaleAle/ddup](https://github.com/ItalyPaleAle/ddup). It must not be part of any upstream PR.

Last reviewed: 2026-10-16 (dates are UTC). This fork's `main` is upstream `main` (`1ade733`, which has #35, #36 and #41) with the open upstream pull requests #37 to #40 merged, and the fork-only features below on top; see "State of the branch".
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
- Upstream has a `DEVELOPMENT.md` and Makefile targets ([#36](https://github.com/ItalyPaleAle/ddup/pull/36), merged): use them for the checks. The plain commands in the checklist at the end do the same.
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
| [#35](https://github.com/ItalyPaleAle/ddup/pull/35) | Version footer (issue #28): build info written into `index.html` by the server, reload on upgrade | **Merged** 2026-10-15 (`1472e67`) after the maintainer's own changes | `feat/version-footer` | `pkg/server/buildinfo.go` and `static.go` are upstream's |
| [#36](https://github.com/ItalyPaleAle/ddup/pull/36) | `DEVELOPMENT.md` and Makefile targets (issue #32) | **Merged** 2026-10-15 (`b4ec5fe`) | `docs/development` | Same |
| [#37](https://github.com/ItalyPaleAle/ddup/pull/37) | Endpoint history popover: since when healthy or failing, and the failed checks of an endpoint in the warning state (issue #31) | **Open**, changes requested; the follow-up commits answer the review (seconds, `RWMutex`, the failures in the warning state), and the provider line is removed from the domain card (configuration, not status). The branch was force-pushed, so it is merged here through a bridge commit that records the old head | `feat/endpoint-history` | The "since" tooltip, now the popover |
| [#38](https://github.com/ItalyPaleAle/ddup/pull/38) | `cname` endpoints and `providerOptions.cloudflare.proxied` (issue #24) | **Open**, review required; verified against the real Cloudflare API | `feat/cname-proxied` | `cname` and `proxied` on the endpoint, typed targets |
| [#39](https://github.com/ItalyPaleAle/ddup/pull/39) | Public IP lookup over HTTPS, `ipLookupFamily`, `ipLookupURLs`, no built-in services (issue #25); verified against the real Cloudflare API | **Open**, changes requested; simplified after review (no `pattern`, an endpoint with `ipLookupURLs` and no family looks up IPv4, the lookup options are a struct) | `feat/ip-lookup-https` | Per-endpoint `ipLookup: {urls, family, pattern}` |
| [#40](https://github.com/ItalyPaleAle/ddup/pull/40) | Notifications for ntfy, webhooks and email, on go-kit v1.5.0 (issue #26) | **Open**, changes requested; reworked after review (ntfy is a webhook `format`, no redaction of provider secrets, one email sender per recipient). The fork has the reworked head: `notifications.ntfy` is now a `webhooks` entry with `format: ntfy` | `feat/notifications` | Templated `webhooks` with `vars` |
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

## State of the branch `integration/fork`

A test of "the fork as it would be if upstream merged all our open PRs as they are". It is upstream `main` (`23ac0bb`) with #35 to #40 merged (branch `integration/open-prs`, with the integration fixes described in `scratch/integration/conflicts.md`), and the fork-only features on top, one commit each.
`main` is not changed by it. The deployed configuration has to change to run it: see `scratch/integration/cluster-migration.md`.

### Fork-only on top

| Feature | Where | Status |
| :------ | :---- | :----- |
| Leader election and standby request forwarding | `pkg/leader`, `pkg/server/leaderproxy.go`, `leaderElection` | Declined (#30). Fork-only on purpose; every other feature must work with it off. Notifications are gated by `isLeader` too; a standby tracks outages without announcing them; the forwarded response carries the standby's own `X-Ddup-Build` |
| Theme switcher (system, light, dark) | `dashboard/src/lib/theme.ts` | Declined (#20). Fork-only on purpose |
| Endpoint rows: names, priority, published or standby, CNAME and proxied chips, failed-check chip, legend, ordered by priority; `name`, `priority` and `active` in the status API | dashboard, `pkg/healthcheck/status.go`, `endpointnames.go` | Issue [#27](https://github.com/ItalyPaleAle/ddup/issues/27), after #38 to #40. Not a PR yet |
| Detailed notifications: endpoint names in titles, status emoji, per-endpoint errors, priority tiers and `standby` | `pkg/notify/detail.go`, `pkg/healthcheck/notifydetail.go`, `notifications.detail` | Fork extension on top of #40, on by default. Candidate for upstreaming after #40 merges |
| `FORK.md`, `screenshots/` (stale, see below), this file, the fork's README sections | | Fork-only on purpose |
| The regression test of the deployed configuration | `pkg/config/testdata/deployment-like.yaml`, `pkg/healthcheck/deployment_test.go` | Fork-only; keep it passing |

### What the open PRs replace

| Fork code | Replaced by |
| :-------- | :---------- |
| `GET /api/info` and the footer fetched at runtime | #35 (server-side injection, `X-Ddup-Build`, reload on upgrade) |
| The "since" tooltip | #36 is docs only; #37 (History popover) |
| `proxied` on the endpoint, `Target.RecordType()` guessing | #38 (`providerOptions.cloudflare.proxied`, explicit `Target.Type`, `Capabilities()`) |
| Per-endpoint `ipLookup`, the sequential fallback, the lookup cache | #39 (`ipLookupFamily`, `ipLookupURLs`, `pkg/iplookup`, raced services) |
| Templated `webhooks` with `vars` | #40 (typed `notifications`, go-kit 1.5.0) |

### Behaviors the fork loses or changes

Intended (an upstream PR replaces them): templates, headers, custom method and the JSON catch-all receiver of the webhooks; email retries; the backoff (fixed `retryInterval`); ntfy priority urgent (5) for an outage; private destinations need `allowPrivateDestinations`; services of an IP lookup are raced, https only, no redirects, no proxy variables, no shared cache; `since` is a popover; the footer has no build date; `/api/info` is gone.
The tier line, `standby`, the per-endpoint errors and the status emoji were lost too, and came back as the fork extension `notifications.detail`.
Also from upstream `main`, not from the open PRs: `expectStatus` is one code or `2xx` (the fork accepted a list, ranges and classes).
Endpoint names in notification titles (`0167787`) are back with `notifications.detail`: configured names, and the address for an endpoint whose name is only its health-check URL.
Decisions for the user: a build date in the footer, a name in the History popover (it lists addresses), `IPv4 lookup` shown as the name of an unnamed lookup endpoint (the fork hid it), `priority` and `active` omitted from the status API when zero or false.

### Rebase to-do list, from the integration

- #37 should key its times by the published target (`Endpoint.Target()`, or the looked-up address), not `Endpoint.IP`, which is empty for `cname` and dynamic endpoints; its tests need `withoutTimes` once #38 adds `Type`.
- #39 should be written against `Endpoint.Target()`, use typed targets in `checkAndUpdateDNS`, say "ip, cname or ipLookupFamily" in its messages, and skip dynamic endpoints in the duplicate-target check of #38.
- #40 should read the published address of a result (the looked-up address or the CNAME), not `Endpoint.IP`, in `notifyEndpointStates`; `Validate` must assign `err =` where #39 already declared it.
- Each PR appends fields and methods to `domainChecker` in the same place: moving them to their own files (`since.go`, `lookups.go`, `notifications.go`) avoids most conflicts. Recommended merge order: 36, 35, 37, 38, 39, 40.
- #35, #36 and #37 were written on `e02794b`; #38 to #40 on `23ac0bb`.
- The fork's code to keep when they merge: `isLeader` around DNS updates and `notifyOutage`, `updatePending` (set after a standby or a failed update, cleared when an update succeeds), the endpoint names and `active` in the status.

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
The tags are `v0.6.0-fork.N`. The latest is `v0.6.0-fork.24` (2026-10-14), and it is what is deployed.
`main` is ahead of it by the refresh of 2026-10-16: the final heads of #35, #36 and #37 (the failed checks in the history popover), #39 and #40 (both reworked after review) and `1ade733` of upstream. The notifications configuration changes with it: `notifications.ntfy` becomes a `webhooks` entry with `format: ntfy`, so the deployed configuration and the image tag must change together.

| Tag | Added |
| :-- | :---- |
| `fork.17` | Version footer (the first signed tag; the earlier ones are unsigned) |
| `fork.18` to `fork.20` | Leader election, its defaults, request forwarding |
| `fork.21` | Webhook `vars` |
| `fork.22` | Refreshed screenshots (docs only) |
| `fork.23` | The integration of the open upstream PRs (#14, #17); the image build was cancelled for running too long |
| `fork.24` | Container image built for the Linux platforms only |

## Before opening a PR

1. Open an issue first if it is a new feature, and wait for agreement.
2. `git fetch upstream`, then branch from `upstream/main`.
3. Cherry-pick only the commits for one feature, and remove anything fork-only (see above).
4. Run the checks (`DEVELOPMENT.md` and the Makefile targets say how; these are the plain commands):
   - `mkdir -p dashboard/dist && touch dashboard/dist/.placeholder` (the Go code embeds `dashboard/dist`; it is gitignored)
   - `go vet -tags unit ./...` and `go vet -tags unit,dashboarddev ./cmd/...`
   - `gofmt -l pkg cmd` lists nothing
   - `go test -race -tags unit ./pkg/...`
   - `golangci-lint run` with the version in `.golangci-lint-version`
   - For dashboard changes, in `dashboard`: `pnpm install --frozen-lockfile`, `pnpm type-check lint format:check build` under Node 24, and `pnpm test:e2e` (needs `pnpm exec playwright install chromium` once), plus screenshots of the change
5. Make sure the commits are signed (`git log --show-signature`), and that the code and the PR text follow `AGENTS.md`.
6. One focused PR, not stacked, with a description that says how it was tested.
