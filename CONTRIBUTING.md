# Contributing to WinRift

WinRift is a public source portfolio with a private production deployment. Issues and technical discussion are welcome. Code contributions should begin with an issue or maintainer conversation so work does not conflict with the product direction, Riot API constraints, or the source-available license.

## Before Opening an Issue

- Search existing issues and pull requests.
- Do not include Riot API keys, player identifiers that are not your own, private server addresses, credentials, raw match payloads, or exploit details.
- Use a private GitHub security advisory for vulnerabilities.
- For performance reports, include the route, patch, cache header, measured latency, and whether the request was a first load or repeat load.

## Development Checks

Core changes:

```bash
cd core
go test ./...
go vet ./...
```

Web changes:

```bash
cd apps/web
npm ci
npm test
npm run build
```

Changes to champion-page loading, cache keys, read models, or deployment behavior should also run the relevant performance smoke documented in [Performance Guardrails](docs/product/performance-guardrails.md).

## Pull Requests

- Keep the change focused and explain the user-visible or operational outcome.
- Add or update tests for behavior changes.
- Update current documentation when configuration, deployment, API contracts, or performance expectations change.
- Preserve historical measurements in dated audit documents.
- Do not commit generated build output, runtime data, secrets, or private infrastructure details.
- Wait for all required checks before requesting merge.

Because WinRift is source-available rather than open source, unsolicited code may require separate written contribution terms before it can be accepted. Opening an issue or pull request does not grant permission to use, deploy, redistribute, or create derivatives of WinRift beyond the rights in [LICENSE](LICENSE).
