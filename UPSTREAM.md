# Upstream tracking

This file is specific to this fork: it tracks what has been, and still has to be, submitted to
[ItalyPaleAle/ddup](https://github.com/ItalyPaleAle/ddup). It must not be part of any upstream PR.

Last reviewed: 2026-10-03. Upstream `main` is `7bfcb68` (it includes #6, #9, #15 and #16, all squashed).
This fork's `main` records upstream as merged (`git merge -s ours`), because upstream squashes and the histories
differ. **Always branch from `upstream/main` and apply the listed changes**; never open a PR from this fork's
`main`. When upstream moves, port what the maintainer changed by hand (they push their own commits onto our PR
branches: check `git log upstream/main` and the PR branches), then record the merge again with `-s ours`.

## Upstream PRs

| Upstream PR | What | Status |
| :---------- | :--- | :----- |
| #6          | Health check `method`, `expectStatus`, `recoverAfter`, transport race fix | **Merged** (maintainer simplified `expectStatus` to one code or `2xx`; the fork has the same) |
| #16         | Document the Cloudflare token permissions | **Merged** |
| #9          | "Check now" button and `POST /api/check` | **Merged** (maintainer reworked the header into a split auto-refresh button; ported to the fork) |
| #15         | `UpdateRecords` reports whether anything changed | **Merged** |
| #17         | Cache headers: stop caching the page and icon for 24h | Open |
| #18         | Overall status in the tab icon | Open (the icon design is a placeholder) |
| #19         | Domain health counts as compact pills | Open |
| #20         | Light/dark/system theme switcher | Open |
| #8          | `!env` / `!file` secret references | **Declined** by the maintainer. Fork-only, don't resubmit (branch `pr/3-secret-refs` is a standalone commit if you want to argue for it) |
| #7, #10, #11, #12, #13, #14 | The old stacked PRs | Closed by us, to keep reviews small. Replaced by the branches below |

## Queued work (branches on this fork, in dependency order, not yet PRs)

Open each one only when the one before it has merged, then rebase it onto `upstream/main`. They still sit on
the old copies of #6, #9 and #15, so expect to drop those commits when rebasing.

| Branch | Feature | Notes |
| :----- | :------ | :---- |
| `chain/1-webhooks` | Webhooks (`dns_updated`, `dns_update_failed`, `all_unhealthy`) | **Open question:** the maintainer asked to use `go-kit/webhook` and `go-kit/emailer`. go-kit's client has no custom method, templated body/headers, retry or timeout settings, and blocks LAN destinations (no way to allow them), so using it would drop features. Either contribute those to go-kit first or defend the notifier |
| `chain/2-proxied-targets` | `proxied` Cloudflare records and typed DNS targets | The token docs say records are created DNS-only; update that sentence in this PR |
| `chain/3-priorities` | Failover priorities | Maintainer said priorities and `proxied` could go on their own |
| `chain/4-cname` | CNAME targets | Discuss in upstream issue #3 first: it was rejected there |
| `chain/5-iplookup` | `ipLookup` for dynamic DNS | Use case answered (replaces a DDNS script: ddup already has the credentials and the DNS update logic). Includes the shared-address and URL-redaction fixes |

## Changes on `main` not covered by the above

| Commit on `main` | What | Depends on / notes |
| :--------------- | :--- | :----------------- |
| `b6b0392` | Dashboard: endpoint rows with names and icons for CNAME, proxied, priority, active/standby and failures; legend; endpoints ordered by priority; the status API includes the endpoint name | Needs `chain/3-priorities` (and `chain/2`); the CNAME icon only matters with `chain/4` |
| `bcc2293` | Webhook events carry the domain's health `status` with `.StatusTag` and `.StatusEmoji`; the sample ntfy webhook sets the tag and a higher priority when unhealthy | Needs webhooks |

## Maintainer conventions seen in review

- `UpdateRecords`-style functions use named results; blank line before `result.Changed = true`.
- Tests use `require.ErrorContains` rather than `assert.Contains(err.Error(), ...)`.
- golangci-lint is pinned in `.github/workflows/ci.yaml` (v2.14.0 at the time of writing); run that version.
- One focused PR each; keep new config options minimal (the maintainer pushed back on lists, ranges and extra knobs).

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
