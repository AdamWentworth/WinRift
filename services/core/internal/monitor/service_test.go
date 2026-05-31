package monitor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"winrift/services/core/internal/config"
	"winrift/services/core/internal/runstate"
)

type recordingNotifier struct {
	subjects []string
}

func (n *recordingNotifier) Send(subject, body string) error {
	n.subjects = append(n.subjects, subject)
	return nil
}

func TestCheckReportsRiotAuthMarker(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "riot-auth-failed")
	if err := os.WriteFile(marker, []byte("status=403 route=NA1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	service := NewService(config.Config{
		RiotAuthFailureMarkerPath: marker,
	}, nil)

	issue := service.Check(context.Background())
	if issue.Key != "riot-auth-failed" {
		t.Fatalf("issue key = %q, want riot-auth-failed", issue.Key)
	}
}

func TestCheckReportsStaleWorkerHeartbeat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "heartbeat.json")
	if err := runstate.WriteWorkerHeartbeat(path, runstate.WorkerHeartbeat{
		Timestamp: time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC),
		Status:    "active",
		Platforms: 14,
		Requests:  90,
	}); err != nil {
		t.Fatal(err)
	}
	service := NewService(config.Config{
		RiotAuthFailureMarkerPath:  filepath.Join(dir, "missing-marker"),
		MonitorWorkerHeartbeatPath: path,
		MonitorWorkerStaleAfter:    15 * time.Minute,
	}, nil)
	service.now = func() time.Time {
		return time.Date(2026, 5, 30, 10, 20, 0, 0, time.UTC)
	}

	issue := service.Check(context.Background())
	if issue.Key != "worker-heartbeat-stale" {
		t.Fatalf("issue key = %q, want worker-heartbeat-stale", issue.Key)
	}
}

func TestRunOnceSuppressesDuplicateAlertsUntilCooldown(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "riot-auth-failed")
	if err := os.WriteFile(marker, []byte("status=403\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	notifier := &recordingNotifier{}
	service := NewService(config.Config{
		RiotAuthFailureMarkerPath: marker,
		MonitorAlertStatePath:     filepath.Join(dir, "state.json"),
		MonitorAlertCooldown:      time.Hour,
	}, notifier)
	now := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	service.runOnce(context.Background())
	service.runOnce(context.Background())
	if got := len(notifier.subjects); got != 1 {
		t.Fatalf("notifications = %d, want 1", got)
	}

	now = now.Add(2 * time.Hour)
	service.runOnce(context.Background())
	if got := len(notifier.subjects); got != 2 {
		t.Fatalf("notifications after cooldown = %d, want 2", got)
	}
}
