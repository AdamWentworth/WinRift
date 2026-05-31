# Public Release Readiness

WinRift is intended to become a public resume/project repo, but the private development history should not be made public as-is.

## Current Working Tree

Current public-facing cleanup status:

- Root `.env` is ignored.
- `.env.example` and `ops/prod/winrift.env.example` use placeholders.
- The legacy prototype code and generated legacy data have been removed from the working tree.
- The test fixture needed by Go tests now lives under `core/testdata/` with player identifiers sanitized.
- Runtime data, local database files, dumps, backups, local builds, and generated web output are ignored.

## History Risk

The existing Git history contains old prototype commits. During audit, history showed:

- old `node_modules/` contents,
- old SQLite/database artifacts,
- old generated JSON data,
- old legacy Python/Node scripts,
- dependency fixture private keys from vendored `node_modules`,
- possible hardcoded Riot API key assignment sites in retired prototype scripts,
- hardcoded local/remote database connection references in retired prototype scripts.

That does not mean a currently valid secret is exposed, but it is enough risk that the safest public release path is a clean-history public repo.

## Recommended Public Strategy

Use this private repo as the development repo. Create a separate public repo from a sanitized snapshot:

1. Start from a clean working tree.
2. Run tests and build checks.
3. Copy the current source tree without `.git`, `.env`, local volumes, build output, or ignored files.
4. Initialize a new public Git repo with one clean initial commit.
5. Add a license intentionally. If unsure, leave it unlicensed until you choose one.
6. Run a secret scanner before the first public push.
7. Push the clean-history repo publicly.

This avoids trying to surgically rewrite years of messy prototype history.

## Alternative: Rewrite History

If keeping stars/issues/history ever matters, use a proper history rewrite before making this repo public:

- `git filter-repo` or BFG Repo-Cleaner for data and dependency artifacts,
- a real secret scanner such as Gitleaks or TruffleHog,
- forced push to a new remote,
- rotation of any credential that may have ever appeared in history.

This is more delicate than the clean-snapshot approach and not necessary for a resume link.

## Pre-Public Checklist

- Confirm `git status --short` is clean.
- Confirm no tracked `.env` files except examples.
- Confirm no tracked `legacy/`, `node_modules/`, `dist/`, database, dump, backup, or unsanitized raw match data files.
- Run backend tests: `cd core && go test ./...`.
- Run frontend tests/build: `cd apps/web && npm test && npm run build`.
- Run a secret scanner with redaction enabled.
- Review GitHub Actions before public release. CI is fine, but production deploy workflows should remain manual and bound to private self-hosted runner labels/secrets.
- Decide whether production ops docs should stay public or be trimmed to avoid exposing home-network operational details.

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
