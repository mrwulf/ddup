# Upstream tracking

This file is specific to this fork: it tracks what has been, and still has to be, submitted to
[ItalyPaleAle/ddup](https://github.com/ItalyPaleAle/ddup). It must not be part of any upstream PR.

Last reviewed: 2026-10-13. Upstream `main` is `c5b47cf`: it includes #6, #9, #15, #16, #17 and #21, all squashed, plus go-kit 1.4.0 (`!file` and `!envfile` in the config, see #22) and the maintainer's `AGENTS.md`.
The maintainer plans a release once #18 and #19 are merged; features go in the release after that.
This fork's `main` records upstream as merged (`git merge -s ours`), because upstream squashes and the histories
differ. **Always branch from `upstream/main` and apply the listed changes**; never open a PR from this fork's
`main`. When upstream moves, port what the maintainer changed by hand (they push their own commits onto our PR
branches: check `git log upstream/main` and the PR branches, for example the "💄" commits), then record the merge
again with `-s ours`.

## Rules from the maintainer

- **Commits must be signed.** The maintainer said they will enforce it. Signing is set up on this machine (SSH
  key `~/.ssh/github_signing`, registered on GitHub as a signing key; `commit.gpgsign` and `tag.gpgsign` are on).
  Commits before `v0.6.0-fork.17` and the earlier tags are unsigned.
- **Open an issue before sending a PR for a new feature**, and wait for agreement. Bug fixes can go straight to a PR.
- Show how a UI change looks (screenshots) and offer design options for visual choices.
- Style: upstream now has an [`AGENTS.md`](https://github.com/ItalyPaleAle/ddup/blob/main/AGENTS.md); read it before every PR. In short:
  one sentence per line in comments, never wrapped, and no trailing period on single-line comments; never declare variables inside
  `if` conditions; always braces around control flow bodies (TS/JS); pnpm, not npm; every enabled `<button>` needs `cursor: pointer`;
  run Go tests with `-tags unit`, and never put the `unit` tag in a `_test.go` file; run `make gen-config` when `pkg/config.Config`
  or a struct it references changes (every config feature PR below does; the target runs `./tools/gen-config`, which isn't in upstream's repository, asked in [#32](https://github.com/ItalyPaleAle/ddup/issues/32)); don't stage or unstage changes unless asked.
  Also, from review: struct literals with one field per line; named results for `UpdateRecords`-style functions; blank line before
  `result.Changed = true`; `require.ErrorContains` instead of `assert.Contains(err.Error(), ...)`; run `gofmt`.
- golangci-lint is pinned in `.golangci-lint-version`; install that release binary (package manager builds refuse this module's Go version).
- One focused PR each, not stacked. Keep new config options minimal: the maintainer pushed back on lists, ranges
  and extra knobs, on env/file secrets, and on CNAME targets.

## Upstream PRs

| Upstream PR | What | Status |
| :---------- | :--- | :----- |
| #6          | Health check `method`, `expectStatus`, `recoverAfter`, transport race fix | **Merged** (maintainer simplified `expectStatus` to one code or `2xx`; the fork has the same) |
| #16         | Document the Cloudflare token permissions | **Merged** |
| #9          | "Check now" button and `POST /api/check` | **Merged** (maintainer reworked the header into a split auto-refresh button; ported to the fork) |
| #15         | `UpdateRecords` reports whether anything changed | **Merged** |
| #17         | Cache headers: stop caching the page and icon for 24h | **Merged** |
| #21         | Health checks: open a new connection for every check (keep-alive made checks reuse one connection forever) | **Merged** (maintainer pushed a style-only "💄" commit, ported to the fork). Reminder to sign commits |
| #18         | Overall status in the tab icon | **Open, updated 2026-10-13.** The maintainer chose the round "d" with the shape badge (check, warning triangle, cross; a grey circle with a dash for unknown) and asked for an SVG of the icon without the badge, which is now `dashboard/public/favicon.svg`, and for the icon in the header, with its status. Done in `e80cb64` on `pr/13-tab-icon-status`, replied. The README screenshot is not changed in this PR, because #19 changes `screenshot.webp` and two PRs on one binary file would conflict: after both merge, check that `screenshot.webp` shows the header icon. Fork line: `feat/tab-icon-shapes` |
| #19         | Domain health counts as compact pills | **Open, updated 2026-10-13.** The maintainer chose design B, a segmented bar under the search bar that gets smaller with the screen. Done in `a215e43` on `pr/14-summary-pills`, with a new README `screenshot.webp` in `f85c255`, replied. Fork line: `feat/summary-bar` |
| #34         | Failover priorities for endpoints (issue #23) | **Open, 2026-10-13**, no review yet. A standalone version on `upstream/main`: priorities only, no CNAME, proxied or dashboard changes. Branch `feat/priorities` |
| #35         | Version of the build in the dashboard footer (issue #28) | **Open, 2026-10-13**, no review yet. `GET /api/info` plus a muted footer line, from upstream's `buildinfo`. Branch `feat/version-footer` |
| #20         | Light/dark/system theme switcher | **Declined** (closed by the maintainer): causes theme flicker, and respecting the browser preference is the right choice. Fork-only, don't resubmit |
| #8          | `!env` / `!file` secret references | **Declined**, then replaced by upstream's own `!file` and `!envfile` (go-kit 1.4.0, issue #22). The fork dropped `SecretString` and uses upstream's tags; `!env` no longer exists |
| #7, #10, #11, #12, #13, #14 | The old stacked PRs | Closed by us, to keep reviews small. Replaced by the branches below |

## Queued work (branches on this fork, in dependency order, not yet PRs)

The issues are open (2026-10-12), with [#29](https://github.com/ItalyPaleAle/ddup/issues/29) as the tracking issue.
The first three are requirements for this deployment: if upstream declines them, the fork continues on its own and stops sending other changes (said plainly in #29, and fine either way).
Open each PR only after the maintainer agrees in its issue and the one before it has merged, then rebase it onto
`upstream/main`. They still sit on the old copies of #6, #9 and #15, so expect to drop those commits when rebasing,
and re-sign the commits (they are unsigned).

| Branch | Feature | Notes |
| :----- | :------ | :---- |
| `chain/1-webhooks` ([#26](https://github.com/ItalyPaleAle/ddup/issues/26), priority 4) | Webhooks (`dns_updated`, `dns_update_failed`, `all_unhealthy`) | **Open question:** the maintainer asked to use `go-kit/webhook` and `go-kit/emailer`. go-kit's client has no custom method, templated body/headers, retry or timeout settings, and blocks LAN destinations (no way to allow them), so using it would drop features. Either contribute those to go-kit first or defend the notifier |
| `chain/3-priorities` ([#23](https://github.com/ItalyPaleAle/ddup/issues/23), priority 1) | Failover priorities | Required. **Welcomed by the maintainer** ("This is a good idea!", 2026-10-13). A standalone version on `upstream/main` is upstream PR [#34](https://github.com/ItalyPaleAle/ddup/pull/34) (branch `feat/priorities`, no review yet). The fork's own version stays stacked on typed targets until #34 and the Cloudflare PR land |
| `chain/2-proxied-targets` and `chain/4-cname` ([#24](https://github.com/ItalyPaleAle/ddup/issues/24), priority 2) | `proxied` Cloudflare records, typed DNS targets, CNAME targets, and the dashboard icons for proxied and CNAME (from `b6b0392`) | Required; one PR after priorities. Update the README sentence about DNS-only records. The CNAME case (Cloudflare Tunnel as a last resort) answers the questions in the closed issue #3 |
| `feat/leader-election` ([#30](https://github.com/ItalyPaleAle/ddup/issues/30), deployed since fork.18) | Optional Lease-based leader election for several replicas, and standbys forwarding API requests to the leader | **Declined 2026-10-13** (closed as not planned): too much complexity to maintain, ddup is not in the critical path (a crash restarts in about a second), and a standby does not replicate state. Fork-only, don't resubmit. The fork keeps it, and the cluster runs two replicas with it |
| `chain/5-iplookup` ([#25](https://github.com/ItalyPaleAle/ddup/issues/25), priority 3) | `ipLookup` for dynamic DNS | Required. Use case answered (replaces a DDNS script: ddup already has the credentials and the DNS update logic). Includes the shared-address and URL-redaction fixes. **Add the no-keep-alive lookup client** (`lookupClient`, the "use a new connection for every lookup" commit on `main`) when rebasing |

## Built, waiting for the issue to be agreed

| Branch | Feature | Notes |
| :----- | :------ | :---- |
| `feat/endpoint-since` ([#31](https://github.com/ItalyPaleAle/ddup/issues/31)) | Dashboard tooltip with "healthy since" or "failing since" and the last failure, from in-memory timestamps per endpoint (no history); the status API gets `since`, `sinceStart` and `lastFailure` | Built, not yet a PR; it sits on `feat/webhook-vars`, so rebase onto `upstream/main` once the issue is agreed (it needs the endpoint rows from [#27](https://github.com/ItalyPaleAle/ddup/issues/27)) |

## Ideas filed upstream, not built yet

| Issue | What |
| :---- | :--- |
| [#32](https://github.com/ItalyPaleAle/ddup/issues/32) | Optional `mise.toml` (or `make check`) that runs what CI runs; also asks about the missing `tools/gen-config` and a `v` in `AGENTS.md`'s install snippet. Built and passing on branch `feat/mise` (from `upstream/main`, one signed commit), waiting for the issue to be agreed |

## Changes on `main` not covered by the above

| Commit on `main` | What | Depends on / notes |
| :--------------- | :--- | :----------------- |
| `b6b0392` | Dashboard: endpoint rows with names and icons for CNAME, proxied, priority, active/standby and failures; legend; endpoints ordered by priority; the status API includes the endpoint name | Split: the proxied and CNAME icons go with [#24](https://github.com/ItalyPaleAle/ddup/issues/24); the rest (priority, standby, failures, legend) is [#27](https://github.com/ItalyPaleAle/ddup/issues/27), after priorities |
| `3b95f32` | Dashboard: the endpoint icons stay on the right of the row, also when they wrap to their own line | Belongs with `b6b0392` |
| `bcc2293` | Webhook events carry the domain's health `status` with `.StatusTag` and `.StatusEmoji`; the sample ntfy webhook sets the tag and a higher priority when unhealthy | Needs webhooks |
| `0167787` | Webhook titles (`.Subject`) name the published endpoints (`tunnel`, `vps-eu`) instead of their addresses, falling back to the address for unknown targets | Needs webhooks. Redo it on top of `chain/1-webhooks` |
| `76e475d` | Dashboard footer with the version of the build (`ddup <version> · built <date> · <commit>`), from a new `GET /api/info` | Standalone; issue [#28](https://github.com/ItalyPaleAle/ddup/issues/28), **approved by the maintainer** ("Sure!", 2026-10-13). Upstream PR [#35](https://github.com/ItalyPaleAle/ddup/pull/35) (branch `feat/version-footer`, one signed commit on `upstream/main`, no review yet), written against upstream's `buildinfo` package, so the fork's own version stays until it merges |
| `17ed6e5` | IP lookups open a new connection every time (the lookup client has no keep-alives) | Needs `chain/5-iplookup`; goes with it |
| `76f9366` | Light/dark/system theme switcher | Declined upstream (#20). Fork-only |
| `37b18db`, `9f8ebfa` | Tab icon with the overall status | Submitted as #18, waiting on the design feedback above |
| `9e6604d` | Summary pills | Submitted as #19, waiting on the design feedback above |

## Fork-only on purpose

- Leader election (declined upstream, see #30): `pkg/leader`, the `leaderElection` config, and the standby request forwarding in `pkg/server`. Every fork feature must keep working with it off.

- Webhook `vars` (values for the templates, loaded with upstream's `!file` and `!envfile`), which replaced the fork's `env` template function. Upstream issue [#33](https://github.com/ItalyPaleAle/ddup/issues/33) (needs the webhooks PR first).
- The theme switcher (declined upstream, see #20).
- CNAME targets, until there's agreement in issue [#24](https://github.com/ItalyPaleAle/ddup/issues/24).
- `FORK.md`, its `screenshots/` (sample data only), and the README's screenshots and link to `FORK.md` (the README image replaced upstream's `screenshot.webp`, which this fork deleted). Keep them out of upstream PRs.
- Release tags (`v0.6.0-fork.N`) and everything that publishes the fork's container image.

## Before opening a PR

1. Open an issue first if it is a new feature, and wait for agreement.
2. `git fetch upstream`, then branch from `upstream/main`.
3. Cherry-pick only the commits for one feature, and remove anything fork-only (see above).
4. `golangci-lint` (the version in `.golangci-lint-version`), `make gen-config` if a config struct changed, `go test -race -tags unit ./pkg/...`, and for
   dashboard changes `pnpm type-check lint format:check build` under Node 24, plus screenshots of the change.
5. Make sure the commits are signed (`git log --show-signature`).
6. One focused PR each, not stacked.
