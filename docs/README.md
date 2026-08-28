# WinRift Documentation

This directory contains current technical references for the running WinRift system. Historical audits, completed roadmaps, public-project boilerplate, and superseded design journals are intentionally excluded.

## System and Operations

| Reference | Purpose |
|---|---|
| [Data Dictionary](data-dictionary.md) | ClickHouse tables and compiled read models |
| [Collector Runbook](collector-runbook.md) | Collection, Riot-key rotation, patch rollover, and worker health |
| [Storage Policy](storage-policy.md) | Raw retention and archived summary behavior |
| [ClickHouse Queries](clickhouse-queries.md) | Owner diagnostics and operational queries |
| [Production Operations](../ops/prod/README.md) | Deployment, rollback, health checks, and server commands |

## Product and Analytics

| Reference | Purpose |
|---|---|
| [Analytics Philosophy](product/analytics-philosophy.md) | Product claims, uncertainty, and live-game boundaries |
| [Build Guides](product/build-guides.md) | Guide and matchup-build behavior |
| [Tier-List Ranking](product/tier-list-ranking.md) | Role ranking and tier assignment |
| [Win-Condition Validation](product/win-condition-validation.md) | Five-axis validation method and diagnostics |

## Quality

| Reference | Purpose |
|---|---|
| [Performance Guardrails](product/performance-guardrails.md) | API, cache, prewarm, and browser-route gates |

Current runbooks and executable configuration are authoritative. Patch examples are illustrative unless explicitly labeled as the current production target.
