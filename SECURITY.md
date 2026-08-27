# Security Policy

WinRift is a source-available analytics project with a private-LAN production deployment. The current monitoring and deployment controls are designed for that trusted-network boundary; they are not a substitute for public authentication, edge abuse protection, or internet-facing threat monitoring.

## Supported Branch

| Branch | Status |
|--------|--------|
| `master` / `0.1.x` | Supported |

## Reporting A Vulnerability

Please [open a private GitHub security advisory](https://github.com/AdamWentworth/WinRift/security/advisories/new), or contact the repository owner directly through GitHub.

Do not open a public issue containing:

- Riot API keys
- ClickHouse credentials
- private server IPs, domains, or SSH details
- exploit payloads against a live deployment

## Secrets

Real secrets belong only in local or server `.env` files. The committed `.env.example` and `ops/prod/winrift.env.example` files contain placeholders only.

The collector is designed to stop on Riot API 401/403 responses when `RIOT_AUTH_FAILURE_EXIT=true`, and the API should continue serving non-Riot-dependent endpoints while the key is refreshed.

## Public Deployment Notes

Before exposing WinRift beyond a private network, add:

- authentication or strict public API scoping,
- edge rate limiting,
- request logging and alerting,
- dependency/security scan review,
- an explicit Riot policy review for public live-game UX.
