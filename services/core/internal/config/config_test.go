package config

import (
	"testing"
	"time"
)

func TestCollectorUsableRequestsPerRegion(t *testing.T) {
	cfg := Config{
		CollectorInterval:          120 * time.Second,
		CollectorRateLimitRequests: 100,
		CollectorRateLimitWindow:   120 * time.Second,
		CollectorRateLimitReserve:  10,
	}
	if got := cfg.CollectorUsableRequestsPerRegion(); got != 90 {
		t.Fatalf("usable requests = %d, want 90", got)
	}

	cfg.CollectorInterval = 60 * time.Second
	if got := cfg.CollectorUsableRequestsPerRegion(); got != 40 {
		t.Fatalf("scaled usable requests = %d, want 40", got)
	}
}

func TestCollectorRankRequestBudget(t *testing.T) {
	cfg := Config{
		RankEnrichmentEnabled:     true,
		RankEnrichmentMaxRequests: 5,
	}
	if got := cfg.CollectorRankRequestBudget(23); got != 5 {
		t.Fatalf("rank budget = %d, want 5", got)
	}
	if got := cfg.CollectorRankRequestBudget(3); got != 2 {
		t.Fatalf("small rank budget = %d, want 2", got)
	}

	cfg.RankEnrichmentEnabled = false
	if got := cfg.CollectorRankRequestBudget(23); got != 0 {
		t.Fatalf("disabled rank budget = %d, want 0", got)
	}
}

func TestCollectorAccountAliasRequestBudget(t *testing.T) {
	cfg := Config{
		AccountAliasEnrichmentEnabled: true,
		AccountAliasMaxRequests:       3,
	}
	if got := cfg.CollectorAccountAliasRequestBudget(23); got != 3 {
		t.Fatalf("alias budget = %d, want 3", got)
	}
	if got := cfg.CollectorAccountAliasRequestBudget(2); got != 1 {
		t.Fatalf("small alias budget = %d, want 1", got)
	}

	cfg.AccountAliasEnrichmentEnabled = false
	if got := cfg.CollectorAccountAliasRequestBudget(23); got != 0 {
		t.Fatalf("disabled alias budget = %d, want 0", got)
	}
}

func TestLoadPatchRetentionConfig(t *testing.T) {
	t.Setenv("COLLECTOR_CURRENT_PATCH", "16.10")
	t.Setenv("COLLECTOR_PATCH_RETENTION_COUNT", "2")
	t.Setenv("COLLECTOR_PRUNE_OLD_PATCHES_ON_START", "true")

	cfg := Load()
	if cfg.CollectorCurrentPatch != "16.10" {
		t.Fatalf("current patch = %q, want 16.10", cfg.CollectorCurrentPatch)
	}
	if cfg.CollectorPatchRetention != 2 {
		t.Fatalf("patch retention = %d, want 2", cfg.CollectorPatchRetention)
	}
	if !cfg.CollectorPruneOldPatches {
		t.Fatal("expected prune old patches on start")
	}
}

func TestLoadMonitorConfig(t *testing.T) {
	t.Setenv("MONITOR_API_HEALTH_URL", "http://api:8000/api/health")
	t.Setenv("MONITOR_INTERVAL_SECONDS", "30")
	t.Setenv("MONITOR_WORKER_HEARTBEAT_PATH", "/run/winrift/worker.json")
	t.Setenv("MONITOR_WORKER_REQUIRED", "true")
	t.Setenv("MONITOR_WORKER_STALE_AFTER_MINUTES", "20")
	t.Setenv("MONITOR_ALERT_STATE_PATH", "/run/winrift/alerts.json")
	t.Setenv("MONITOR_ALERT_COOLDOWN_MINUTES", "90")
	t.Setenv("MONITOR_RECOVERY_ALERTS", "false")
	t.Setenv("ALERT_EMAIL_ENABLED", "true")
	t.Setenv("SMTP_HOST", "smtp.example.test")
	t.Setenv("SMTP_PORT", "2525")
	t.Setenv("SMTP_USERNAME", "winrift")
	t.Setenv("SMTP_PASSWORD", "secret")
	t.Setenv("SMTP_FROM", "winrift@example.test")
	t.Setenv("SMTP_TO", "one@example.test,two@example.test")

	cfg := Load()
	if cfg.MonitorInterval != 30*time.Second {
		t.Fatalf("monitor interval = %s, want 30s", cfg.MonitorInterval)
	}
	if cfg.MonitorWorkerHeartbeatPath != "/run/winrift/worker.json" {
		t.Fatalf("heartbeat path = %q", cfg.MonitorWorkerHeartbeatPath)
	}
	if !cfg.MonitorWorkerRequired {
		t.Fatal("expected worker required")
	}
	if cfg.MonitorWorkerStaleAfter != 20*time.Minute {
		t.Fatalf("stale after = %s, want 20m", cfg.MonitorWorkerStaleAfter)
	}
	if cfg.MonitorAlertCooldown != 90*time.Minute {
		t.Fatalf("alert cooldown = %s, want 90m", cfg.MonitorAlertCooldown)
	}
	if cfg.MonitorRecoveryAlerts {
		t.Fatal("did not expect recovery alerts")
	}
	if !cfg.AlertEmailEnabled || cfg.SMTPHost != "smtp.example.test" || cfg.SMTPPort != 2525 || cfg.SMTPUsername != "winrift" || cfg.SMTPPassword != "secret" || cfg.SMTPFrom != "winrift@example.test" {
		t.Fatalf("smtp config not loaded: %+v", cfg)
	}
	if len(cfg.SMTPTo) != 2 || cfg.SMTPTo[0] != "one@example.test" || cfg.SMTPTo[1] != "two@example.test" {
		t.Fatalf("smtp to = %v", cfg.SMTPTo)
	}
}
