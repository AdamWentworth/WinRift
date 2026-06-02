# Public Release Readiness

WinRift is a public resume/project repo, while the live deployment stays private-LAN first.

## Current Working Tree

Current public-facing cleanup status:

- Root `.env` is ignored.
- `.env.example` and `ops/prod/winrift.env.example` use placeholders.
- The legacy prototype code and generated legacy data have been removed from the working tree.
- The test fixture needed by Go tests now lives under `core/testdata/` with player identifiers sanitized.
- Runtime data, local database files, dumps, backups, local builds, and generated web output are ignored.
- The active Git history has been rewritten to the modern rebuild history; old 2023 prototype commits are no longer on `master`.
- The repository has been made public on GitHub; the production deployment remains private.

## History Risk

Earlier audits found old prototype commits and retired generated data in the pre-rewrite history. That risk was handled by replacing the old branch history with the current rebuild history.

Current checks:

- `git log --all -- .env '*.env' '*secret*' '*credential*' 'legacy/**' 'data/**'` shows no tracked legacy/env history in the active branch set.
- Gitleaks was run against the current repository history with redaction enabled on 2026-05-31 and reported no leaks.

Keep rotating development Riot keys normally, because Riot development keys expire by design.

## Public Strategy

Keep the public repository focused on source, architecture, docs, and sanitized screenshots. Keep the production deployment private and keep server `.env` files, database volumes, ClickHouse backups, and runtime state out of Git.

## Public Maintenance Checklist

- Confirm `git status --short` is clean.
- Confirm no tracked `.env` files except examples.
- Confirm no tracked `legacy/`, `node_modules/`, `dist/`, database, dump, backup, or unsanitized raw match data files.
- Run backend tests: `cd core && go test ./...`.
- Run frontend tests/build: `cd apps/web && npm test && npm run build`.
- Run a secret scanner with redaction enabled.
- Review GitHub Actions before public release. CI is fine, but production deploy workflows should remain manual and bound to private self-hosted runner labels/secrets.
- Keep production docs generic: use placeholders such as `SERVER_LAN_IP` rather than exact private network addresses.

## Legacy Ideas Preserved

The retired prototype still contributed useful ideas. These have been preserved in docs and/or rebuilt:

- five-axis win-condition model,
- team score aggregation,
- primary/secondary/fallback strategy framing,
- game-length buckets,
- precomputed read models for fast UI responses,
- live-match summoner search with colored region selection,
- ranked and champion-performance card layout,
- draggable live-match card ordering,
- strategy iconography and letter-rating language.

See [Legacy Win Condition Audit](legacy-win-condition-audit.md) and [Win Conditions](discussions/win-conditions.md).
