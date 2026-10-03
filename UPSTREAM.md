# Upstream tracking

This file is specific to this fork: it tracks what has been, and still has to be, submitted to
[ItalyPaleAle/ddup](https://github.com/ItalyPaleAle/ddup). It must not be part of any upstream PR.

Last reviewed: 2026-10-03. Upstream `main` is `1499afb` (it includes #6, squashed, and #16).
This fork's `main` has diverged because of that squash, so **always branch from `upstream/main` and
cherry-pick the listed commits**; never open a PR from this fork's `main`.

## What happened to the PRs already sent

| Upstream PR | What | Status | Notes |
| :---------- | :--- | :----- | :---- |
| #6          | Health check `method`, `expectStatus`, `recoverAfter`, transport race fix | **Merged** | `75587c4` on the `pr/1-*` branch |
| #16         | Document the Cloudflare token permissions | **Merged** | |
| #9          | "Check now" button and `POST /api/check` | Open | Maintainer LGTM'd `d1430d0`, waiting to merge because of dependencies |
| #7          | Webhooks (`dns_updated`, `dns_update_failed`, `all_unhealthy`) | Closed by us, to split the stack | **Rework needed:** the maintainer asked to use `github.com/italypaleale/go-kit/webhook` for webhooks and `go-kit/emailer` for email, instead of our own notifier |
| #8          | `!env` / `!file` secret references | **Declined** | Env vars are less secure than a config file; file references are unnecessary complexity. Fork-only, don't resubmit |
| #10         | Failover priorities and CNAME/proxied targets | Closed by us, to split the stack | Maintainer: split CNAME support out. It was discussed and rejected in upstream issue #3; discuss it there first. Priorities and `proxied` could go on their own |
| #11         | `ipLookup` for dynamic DNS | Closed by us, to split the stack | Maintainer asked for the use case (answered: replacing a DDNS script, since ddup already has the credentials and the DNS update logic) |
| #12         | Docs for webhooks, token permissions, email examples | Closed by us, to split the stack | Resubmit with the feature each section documents |
| #13         | `ipLookup` fixes (shared address published once, URL secrets redacted) | Closed by us, to split the stack | Goes with `ipLookup` |

## Clean sources for the remaining work

The branches `pr/1-*` to `pr/8-*` on this fork hold one clean commit per feature (the old stack). Prefer
them to the commits on `main`, which also contain lint fixes and merges:

| Branch | Commit | Feature |
| :----- | :----- | :------ |
| `pr/2-webhooks` | `ca8b991` | Webhooks (rework on go-kit/webhook first) |
| `pr/3-secret-refs` | `95faa69` | `!env` / `!file` (declined, fork-only) |
| `pr/4-check-now` | `d1430d0` | Check now (PR #9) |
| `pr/5-tiers-typed-targets` | `df7aa86` | Priorities, CNAME, proxied |
| `pr/6-ip-lookup` | `b814d0a` | `ipLookup` |
| `pr/7-docs` | `77443d9` | Docs |
| `pr/8-iplookup-fixes` | `775c37f`, `96ac4cb` | `ipLookup` fixes |

## Changes made after those PRs (not covered by any branch above)

| Commit on `main` | What | Depends on / notes |
| :--------------- | :--- | :----------------- |
| `b6b0392` | Dashboard: endpoint rows with names, and icons for CNAME, proxied, priority, active/standby and failures; legend; endpoints ordered by priority; the status API includes the endpoint name | Needs the status fields from the priorities work (`priority`, `active`, `type`, `proxied`). The CNAME icon only matters if CNAME support lands; the rest is useful with priorities alone |
| `9e6604d` | Dashboard: the healthy/warning/unhealthy counts are compact pills on the search row instead of three large cards, so less scrolling, especially on phones | Standalone; applies to upstream's dashboard as it is today |
| `37b18db` | Dashboard: overall status in the tab icon (a "d" with a green/yellow/red badge; the title is unchanged) | Standalone. **Contains a separate bug fix worth its own PR:** the page and icon were cached for 24h, so browsers showed the old dashboard after an upgrade (now `no-cache`, with long caching only for hashed `/assets`; `pkg/server/static.go`) |
| `bcc2293` | Webhook events carry the domain's health `status` (healthy, warning, unhealthy) with `.StatusTag` (ntfy green/yellow/red circle tags) and `.StatusEmoji`; the sample ntfy webhook sets the tag and a higher priority when unhealthy | Depends on webhooks. Redo it on top of go-kit/webhook when that rework happens |

## Fork-only on purpose

- `!env` / `!file` and the `SecretString` type it introduced (provider settings, webhook URLs and headers,
  `recordName`). Declined upstream, but this fork's deployment relies on it. Anything submitted upstream
  that touches those fields has to be rewritten with plain strings.
- CNAME targets, until there's agreement on upstream issue #3.
- Release tags (`v0.6.0-fork.N`) and everything that publishes the fork's container image.

## Before opening a PR

1. `git fetch upstream`, then branch from `upstream/main`.
2. Cherry-pick only the commits for one feature, and remove anything fork-only (see above).
3. `golangci-lint` (the version in `.github/workflows/ci.yaml`), `go test -race -tags unit ./pkg/...`, and for
   dashboard changes `pnpm type-check lint format:check build` under Node 24.
4. One focused PR each, not stacked.
