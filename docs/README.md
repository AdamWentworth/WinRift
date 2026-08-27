# WinRift Documentation

WinRift's documentation is split by whether a document describes the current system, an active product decision, or a dated engineering record. Start with the current references below; historical audits intentionally retain the patch numbers and measurements from the day they were written.

## Current System

| Area | Reference |
|------|-----------|
| Architecture and runtime boundaries | [Architecture](architecture.md) |
| Production deployment and rollback | [Deployment and Operations](ops-deployment.md) |
| Collector behavior and Riot-key safety | [Collector Runbook](collector-runbook.md) |
| ClickHouse tables and read models | [Data Dictionary](data-dictionary.md) |
| Raw and summary retention | [Patch Lifecycle](patch-lifecycle.md) and [Storage Policy](storage-policy.md) |
| Current product and engineering priorities | [Next Steps](product/next-steps.md) |

## Performance and Reliability

| Area | Reference |
|------|-----------|
| Enforced API and browser budgets | [Performance Guardrails](product/performance-guardrails.md) |
| Frontend request/cache rules | [Frontend Performance Audit](product/frontend-performance-audit.md) |
| Read-model coverage | [Read-Model Coverage Audit](product/read-model-coverage-audit.md) |
| Public-repository safety | [Public Release Readiness](public-release-readiness.md) |

## Product Design

- [Build Guides](product/build-guides.md)
- [Tier-List Ranking](product/tier-list-ranking.md)
- [Summoner Profiles](product/summoner-profiles.md)
- [Analytics Philosophy](product/analytics-philosophy.md)
- [Global Background System](product/global-background-system.md)
- [Policy-Safe Live UX](policy-safe-live-ux.md)

## Historical Engineering Records

The following documents explain how the current system evolved. They are evidence and decision history, not current setup instructions:

- [Completed Product Work Journal](product/remaining-work.md)
- [Completed Tech-Debt Roadmap](product/tech-debt-roadmap.md)
- [Production Performance Audit — 2026-05-31](product/production-performance-audit-2026-05-31.md)
- [Win-Condition Validation — 2026-05-31](product/win-condition-validation-2026-05-31.md)
- [Legacy Win-Condition Audit](legacy-win-condition-audit.md)

When a historical record conflicts with a current runbook, the current runbook wins.
