# Upstream tracking

This file is specific to this fork: it tracks what has been, and still has to be, submitted to
[ItalyPaleAle/ddup](https://github.com/ItalyPaleAle/ddup). It must not be part of any upstream PR.

Last reviewed: 2026-10-05. Upstream `main` is `c877cd3`: it includes #6, #9, #15, #16, #17 and #21, all squashed.
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
- Style (see also go-kit's [`AGENTS.md`](https://github.com/italypaleale/go-kit/blob/main/AGENTS.md); this repo has
  none): one sentence per line in comments, never wrapped; struct literals with one field per line; never declare
  variables inside `if` conditions; named results for `UpdateRecords`-style functions; blank line before
  `result.Changed = true`; `require.ErrorContains` instead of `assert.Contains(err.Error(), ...)`; run `gofmt`.
- golangci-lint is pinned in `.github/workflows/ci.yaml` (v2.14.0 at the time of writing); run that version.
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
| #18         | Overall status in the tab icon | **Open, feedback to act on:** why the favicon and not the page title? Share screenshots, offer several icon designs (they don't like the current one). Also: open an issue first |
| #19         | Domain health counts as compact pills | **Open, feedback to act on:** pills on the right are too small, but the current ones are too big on mobile. Propose responsive designs: larger on desktop (smaller than the old cards), progressively smaller on small screens; pills are fine on mobile |
| #20         | Light/dark/system theme switcher | **Declined** (closed by the maintainer): causes theme flicker, and respecting the browser preference is the right choice. Fork-only, don't resubmit |
| #8          | `!env` / `!file` secret references | **Declined** by the maintainer. Fork-only, don't resubmit (branch `pr/3-secret-refs` is a standalone commit if you want to argue for it) |
| #7, #10, #11, #12, #13, #14 | The old stacked PRs | Closed by us, to keep reviews small. Replaced by the branches below |

## Queued work (branches on this fork, in dependency order, not yet PRs)

Open each one only when the one before it has merged, after opening an issue for it, then rebase it onto
`upstream/main`. They still sit on the old copies of #6, #9 and #15, so expect to drop those commits when rebasing,
and re-sign the commits (they are unsigned).

| Branch | Feature | Notes |
| :----- | :------ | :---- |
| `chain/1-webhooks` | Webhooks (`dns_updated`, `dns_update_failed`, `all_unhealthy`) | **Open question:** the maintainer asked to use `go-kit/webhook` and `go-kit/emailer`. go-kit's client has no custom method, templated body/headers, retry or timeout settings, and blocks LAN destinations (no way to allow them), so using it would drop features. Either contribute those to go-kit first or defend the notifier |
| `chain/2-proxied-targets` | `proxied` Cloudflare records and typed DNS targets | The token docs say records are created DNS-only; update that sentence in this PR |
| `chain/3-priorities` | Failover priorities | Maintainer said priorities and `proxied` could go on their own |
| `chain/4-cname` | CNAME targets | Discuss in upstream issue #3 first: it was rejected there |
| `chain/5-iplookup` | `ipLookup` for dynamic DNS | Use case answered (replaces a DDNS script: ddup already has the credentials and the DNS update logic). Includes the shared-address and URL-redaction fixes. **Add the no-keep-alive lookup client** (`lookupClient`, the "use a new connection for every lookup" commit on `main`) when rebasing |

## Changes on `main` not covered by the above

| Commit on `main` | What | Depends on / notes |
| :--------------- | :--- | :----------------- |
| `b6b0392` | Dashboard: endpoint rows with names and icons for CNAME, proxied, priority, active/standby and failures; legend; endpoints ordered by priority; the status API includes the endpoint name | Needs `chain/3-priorities` (and `chain/2`); the CNAME icon only matters with `chain/4` |
| `3b95f32` | Dashboard: the endpoint icons stay on the right of the row, also when they wrap to their own line | Belongs with `b6b0392` |
| `bcc2293` | Webhook events carry the domain's health `status` with `.StatusTag` and `.StatusEmoji`; the sample ntfy webhook sets the tag and a higher priority when unhealthy | Needs webhooks |
| `0167787` | Webhook titles (`.Subject`) name the published endpoints (`tunnel`, `vps-eu`) instead of their addresses, falling back to the address for unknown targets | Needs webhooks. Redo it on top of `chain/1-webhooks` |
| `76e475d` | Dashboard footer with the version of the build (`ddup <version> · built <date> · <commit>`), from a new `GET /api/info` | Standalone. New feature, so it needs an issue first; the footer is useful for knowing which build runs after an upgrade |
| `17ed6e5` | IP lookups open a new connection every time (the lookup client has no keep-alives) | Needs `chain/5-iplookup`; goes with it |
| `76f9366` | Light/dark/system theme switcher | Declined upstream (#20). Fork-only |
| `37b18db`, `9f8ebfa` | Tab icon with the overall status | Submitted as #18, waiting on the design feedback above |
| `9e6604d` | Summary pills | Submitted as #19, waiting on the design feedback above |

## Fork-only on purpose

- `!env` / `!file` and the `SecretString` type it introduced (provider settings, webhook URLs and headers,
  `recordName`). Declined upstream, but this fork's deployment relies on it. Anything submitted upstream
  that touches those fields has to be rewritten with plain strings.
- The theme switcher (declined upstream, see #20).
- CNAME targets, until there's agreement on upstream issue #3.
- `FORK.md`, its `screenshots/` (sample data only), and the README's screenshots and link to `FORK.md` (the README image replaced upstream's `screenshot.webp`, which this fork deleted). Keep them out of upstream PRs.
- Release tags (`v0.6.0-fork.N`) and everything that publishes the fork's container image.

## Before opening a PR

1. Open an issue first if it is a new feature, and wait for agreement.
2. `git fetch upstream`, then branch from `upstream/main`.
3. Cherry-pick only the commits for one feature, and remove anything fork-only (see above).
4. `golangci-lint` (the version in `.github/workflows/ci.yaml`), `go test -race -tags unit ./pkg/...`, and for
   dashboard changes `pnpm type-check lint format:check build` under Node 24, plus screenshots of the change.
5. Make sure the commits are signed (`git log --show-signature`).
6. One focused PR each, not stacked.
