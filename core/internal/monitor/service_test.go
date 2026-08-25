package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"winrift/core/internal/config"
	"winrift/core/internal/runstate"
)

type recordingNotifier struct {
	subjects []string
	err      error
}

func (n *recordingNotifier) Send(subject, body string) error {
	n.subjects = append(n.subjects, subject)
	return n.err
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

func TestCheckLogsStaleWorkerHeartbeatWithoutAlert(t *testing.T) {
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
	if issue.Key != "" {
		t.Fatalf("issue key = %q, want no email-worthy issue", issue.Key)
	}
}

func TestCheckTreatsAuthFailedWorkerHeartbeatAsRiotAuthIssue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "heartbeat.json")
	if err := runstate.WriteWorkerHeartbeat(path, runstate.WorkerHeartbeat{
		Timestamp: time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC),
		Status:    "auth_failed",
		Message:   "Riot API key is missing, expired, or not authorized",
	}); err != nil {
		t.Fatal(err)
	}
	service := NewService(config.Config{
		RiotAuthFailureMarkerPath:  filepath.Join(dir, "missing-marker"),
		MonitorWorkerHeartbeatPath: path,
		MonitorWorkerRequired:      true,
		MonitorWorkerContainerName: "winrift_worker",
	}, nil)

	issue := service.Check(context.Background())
	if issue.Key != "riot-auth-failed" {
		t.Fatalf("issue key = %q, want riot-auth-failed", issue.Key)
	}
}

func TestCheckReportsDownWorkerContainer(t *testing.T) {
	service, cleanup := newDockerContainerMonitor(t, false)
	defer cleanup()

	issue := service.Check(context.Background())
	if issue.Key != "worker-container-down" {
		t.Fatalf("issue key = %q, want worker-container-down", issue.Key)
	}
}

func TestCheckSuppressesDownWorkerContainerDuringStartupGrace(t *testing.T) {
	service, cleanup := newDockerContainerMonitor(t, false)
	defer cleanup()
	started := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	service.started = started
	service.now = func() time.Time { return started.Add(30 * time.Second) }
	service.cfg.MonitorStartupGrace = 2 * time.Minute

	issue := service.Check(context.Background())
	if issue.Key != "" {
		t.Fatalf("issue key = %q, want startup grace suppression", issue.Key)
	}
}

func TestCheckSuppressesTransientAPIFailureDuringStartupGrace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "starting", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	started := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	service := NewService(config.Config{
		RiotAuthFailureMarkerPath: filepath.Join(t.TempDir(), "missing-marker"),
		MonitorAPIHealthURL:       server.URL,
		MonitorStartupGrace:       2 * time.Minute,
	}, nil)
	service.started = started
	service.now = func() time.Time { return started.Add(30 * time.Second) }

	if issue := service.Check(context.Background()); issue.Key != "" {
		t.Fatalf("issue key during startup grace = %q, want no alert", issue.Key)
	}

	service.now = func() time.Time { return started.Add(3 * time.Minute) }
	if issue := service.Check(context.Background()); issue.Key != "api-unhealthy-status" {
		t.Fatalf("issue key after startup grace = %q, want api-unhealthy-status", issue.Key)
	}
}

func newDockerContainerMonitor(t *testing.T, running bool) (Service, func()) {
	t.Helper()
	dir := t.TempDir()
	socketPath := filepath.Join(dir, "docker.sock")
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/containers/winrift_worker/json" {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"Name": "/winrift_worker",
				"State": map[string]any{
					"Running":    running,
					"Status":     "exited",
					"ExitCode":   1,
					"FinishedAt": "2026-05-30T10:00:00Z",
				},
			})
		}),
	}
	go func() {
		_ = server.Serve(listener)
	}()

	service := NewService(config.Config{
		RiotAuthFailureMarkerPath:  filepath.Join(dir, "missing-marker"),
		MonitorWorkerRequired:      true,
		MonitorWorkerContainerName: "winrift_worker",
		MonitorDockerSocketPath:    socketPath,
	}, nil)
	return service, func() {
		_ = server.Close()
	}
}

func TestRunOnceSuppressesDuplicateNonAuthAlertsUntilCooldown(t *testing.T) {
	dir := t.TempDir()
	notifier := &recordingNotifier{}
	service := NewService(config.Config{
		RiotAuthFailureMarkerPath:  filepath.Join(dir, "missing-marker"),
		MonitorWorkerRequired:      true,
		MonitorWorkerHeartbeatPath: filepath.Join(dir, "missing-heartbeat.json"),
		MonitorWorkerStaleAfter:    15 * time.Minute,
		MonitorAlertStatePath:      filepath.Join(dir, "state.json"),
		MonitorAlertCooldown:       time.Hour,
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

func TestRunOnceRepeatsRiotAuthFailureAfterCooldown(t *testing.T) {
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

	if err := os.Remove(marker); err != nil {
		t.Fatal(err)
	}
	service.runOnce(context.Background())
	if state := service.readState(); state.ActiveKey != "" {
		t.Fatalf("active key after recovery = %q, want cleared", state.ActiveKey)
	}

	if err := os.WriteFile(marker, []byte("status=401\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	service.runOnce(context.Background())
	if got := len(notifier.subjects); got != 3 {
		t.Fatalf("notifications for new auth incident = %d, want 3", got)
	}
}

func TestRunOnceRetriesFailedDeliveryWithoutRecordingItAsSent(t *testing.T) {
	dir := t.TempDir()
	notifier := &recordingNotifier{err: errors.New("smtp rejected")}
	service := NewService(config.Config{
		RiotAuthFailureMarkerPath:  filepath.Join(dir, "missing-marker"),
		MonitorWorkerRequired:      true,
		MonitorWorkerHeartbeatPath: filepath.Join(dir, "missing-heartbeat.json"),
		MonitorWorkerStaleAfter:    15 * time.Minute,
		MonitorAlertStatePath:      filepath.Join(dir, "state.json"),
		MonitorAlertCooldown:       24 * time.Hour,
		MonitorAlertRetryInterval:  15 * time.Minute,
	}, notifier)
	now := time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }

	service.runOnce(context.Background())
	state := service.readState()
	if state.LastAttemptAt.IsZero() {
		t.Fatal("last attempt was not recorded")
	}
	if !state.LastSentAt.IsZero() {
		t.Fatalf("failed delivery recorded as sent at %s", state.LastSentAt)
	}

	now = now.Add(10 * time.Minute)
	service.runOnce(context.Background())
	if got := len(notifier.subjects); got != 1 {
		t.Fatalf("notifications before retry interval = %d, want 1", got)
	}

	now = now.Add(5 * time.Minute)
	notifier.err = nil
	service.runOnce(context.Background())
	if got := len(notifier.subjects); got != 2 {
		t.Fatalf("notifications after retry interval = %d, want 2", got)
	}
	if state := service.readState(); state.LastSentAt.IsZero() {
		t.Fatal("successful retry was not recorded as sent")
	}
}

func TestRunOnceDoesNotSendRecoveryEmail(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	notifier := &recordingNotifier{}
	service := NewService(config.Config{
		RiotAuthFailureMarkerPath: filepath.Join(dir, "missing-marker"),
		MonitorAlertStatePath:     statePath,
	}, notifier)
	service.writeState(alertState{
		ActiveKey:  "worker-container-down",
		LastSentAt: time.Date(2026, 5, 30, 10, 0, 0, 0, time.UTC),
	})

	service.runOnce(context.Background())
	if got := len(notifier.subjects); got != 0 {
		t.Fatalf("recovery notifications = %d, want 0", got)
	}
	if state := service.readState(); state.ActiveKey != "" {
		t.Fatalf("active key after recovery = %q, want cleared", state.ActiveKey)
	}
}
