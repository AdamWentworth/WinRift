# Security Policy

WinRift is an MVP analytics project in active development. The production-style deployment described in this repository is intended for private-LAN use unless and until public authentication, abuse protection, and operational monitoring are added.

## Supported Branch

| Branch | Status |
|--------|--------|
| `master` | Active development |

## Reporting A Vulnerability

Please open a private security advisory on GitHub if available, or contact the repository owner directly through GitHub.

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
