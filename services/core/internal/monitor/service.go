package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"winrift/services/core/internal/config"
	"winrift/services/core/internal/runstate"
)

type Notifier interface {
	Send(subject, body string) error
}

type Service struct {
	cfg      config.Config
	client   *http.Client
	notifier Notifier
	now      func() time.Time
}

type Issue struct {
	Key     string
	Subject string
	Body    string
}

type alertState struct {
	ActiveKey  string    `json:"activeKey,omitempty"`
	LastSentAt time.Time `json:"lastSentAt,omitempty"`
}

type apiHealth struct {
	Status  string `json:"status"`
	RiotAPI string `json:"riotApi"`
}

func NewService(cfg config.Config, notifier Notifier) Service {
	return Service{
		cfg:      cfg,
		client:   &http.Client{Timeout: 10 * time.Second},
		notifier: notifier,
		now:      func() time.Time { return time.Now().UTC() },
	}
}

func (s Service) Run(ctx context.Context) error {
	interval := s.cfg.MonitorInterval
	if interval <= 0 {
		interval = time.Minute
	}
	log.Printf(
		"monitor start interval=%s api_health_url=%s heartbeat_path=%s stale_after=%s worker_required=%t email_enabled=%t",
		interval,
		s.cfg.MonitorAPIHealthURL,
		s.cfg.MonitorWorkerHeartbeatPath,
		s.cfg.MonitorWorkerStaleAfter,
		s.cfg.MonitorWorkerRequired,
		s.cfg.AlertEmailEnabled,
	)
	s.runOnce(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s Service) runOnce(ctx context.Context) {
	issue := s.Check(ctx)
	state := s.readState()
	if issue.Key == "" {
		if state.ActiveKey != "" {
			log.Printf("monitor recovered previous_issue=%s", state.ActiveKey)
			if s.cfg.MonitorRecoveryAlerts {
				s.send("WinRift recovered", fmt.Sprintf("WinRift monitor recovered from issue %q at %s.", state.ActiveKey, s.now().Format(time.RFC3339)))
			}
			s.writeState(alertState{})
		}
		return
	}

	if !s.shouldSend(state, issue) {
		return
	}
	log.Printf("monitor alert key=%s subject=%q", issue.Key, issue.Subject)
	s.send(issue.Subject, issue.Body)
	state.ActiveKey = issue.Key
	state.LastSentAt = s.now()
	s.writeState(state)
}

func (s Service) Check(ctx context.Context) Issue {
	if issue := s.checkRiotAuthMarker(); issue.Key != "" {
		return issue
	}
	if issue := s.checkAPI(ctx); issue.Key != "" {
		return issue
	}
	if issue := s.checkWorkerHeartbeat(); issue.Key != "" {
		return issue
	}
	return Issue{}
}

func (s Service) checkRiotAuthMarker() Issue {
	marker := strings.TrimSpace(s.cfg.RiotAuthFailureMarkerPath)
	if marker == "" {
		return Issue{}
	}
	body, err := os.ReadFile(marker)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Issue{}
		}
		return Issue{
			Key:     "riot-auth-marker-unreadable",
			Subject: "WinRift cannot read Riot auth marker",
			Body:    fmt.Sprintf("The monitor could not read %s: %v", marker, err),
		}
	}
	detail := strings.TrimSpace(string(body))
	if detail == "" {
		detail = "marker exists, but no detail was written"
	}
	return Issue{
		Key:     "riot-auth-failed",
		Subject: "WinRift Riot API key needs refresh",
		Body: strings.Join([]string{
			"Riot returned an auth failure and WinRift stopped the collector worker.",
			"",
			"Refresh RIOT_API_KEY in /srv/winrift/.env, then redeploy or restart the API and worker.",
			"",
			"Marker detail:",
			detail,
		}, "\n"),
	}
}

func (s Service) checkAPI(ctx context.Context) Issue {
	healthURL := strings.TrimSpace(s.cfg.MonitorAPIHealthURL)
	if healthURL == "" {
		return Issue{}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return Issue{Key: "api-health-url-invalid", Subject: "WinRift API health URL is invalid", Body: err.Error()}
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return Issue{
			Key:     "api-unreachable",
			Subject: "WinRift API health check failed",
			Body:    fmt.Sprintf("GET %s failed: %v", healthURL, err),
		}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Issue{
			Key:     "api-unhealthy-status",
			Subject: "WinRift API health check returned a bad status",
			Body:    fmt.Sprintf("GET %s returned HTTP %d.", healthURL, resp.StatusCode),
		}
	}
	var health apiHealth
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return Issue{
			Key:     "api-health-json-invalid",
			Subject: "WinRift API health response is invalid",
			Body:    fmt.Sprintf("GET %s returned invalid JSON: %v", healthURL, err),
		}
	}
	if health.Status != "ok" {
		return Issue{
			Key:     "api-unhealthy",
			Subject: "WinRift API is not healthy",
			Body:    fmt.Sprintf("GET %s returned status=%q.", healthURL, health.Status),
		}
	}
	if health.RiotAPI == "auth_failed" {
		return Issue{
			Key:     "riot-auth-failed",
			Subject: "WinRift Riot API key needs refresh",
			Body:    "The API reports riotApi=auth_failed. Refresh RIOT_API_KEY in /srv/winrift/.env, then redeploy or restart the API and worker.",
		}
	}
	return Issue{}
}

func (s Service) checkWorkerHeartbeat() Issue {
	staleAfter := s.cfg.MonitorWorkerStaleAfter
	if staleAfter <= 0 {
		return Issue{}
	}
	path := strings.TrimSpace(s.cfg.MonitorWorkerHeartbeatPath)
	if path == "" {
		return Issue{}
	}
	heartbeat, err := runstate.ReadWorkerHeartbeat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && !s.cfg.MonitorWorkerRequired {
			return Issue{}
		}
		return Issue{
			Key:     "worker-heartbeat-missing",
			Subject: "WinRift collector worker heartbeat is missing",
			Body:    fmt.Sprintf("Expected worker heartbeat at %s, but it could not be read: %v", path, err),
		}
	}
	if heartbeat.Timestamp.IsZero() {
		return Issue{
			Key:     "worker-heartbeat-invalid",
			Subject: "WinRift collector worker heartbeat is invalid",
			Body:    fmt.Sprintf("Worker heartbeat at %s has no timestamp.", path),
		}
	}
	age := s.now().Sub(heartbeat.Timestamp)
	if age > staleAfter {
		return Issue{
			Key:     "worker-heartbeat-stale",
			Subject: "WinRift collector worker looks stale",
			Body: fmt.Sprintf(
				"Worker heartbeat at %s is stale.\n\nLast heartbeat: %s\nAge: %s\nStatus: %s\nPlatforms: %d active of %d\nRequests in last sweep: %d",
				path,
				heartbeat.Timestamp.Format(time.RFC3339),
				age.Round(time.Second),
				heartbeat.Status,
				heartbeat.ActivePlatforms,
				heartbeat.Platforms,
				heartbeat.Requests,
			),
		}
	}
	return Issue{}
}

func (s Service) shouldSend(state alertState, issue Issue) bool {
	if state.ActiveKey != issue.Key {
		return true
	}
	cooldown := s.cfg.MonitorAlertCooldown
	if cooldown <= 0 {
		cooldown = 6 * time.Hour
	}
	return state.LastSentAt.IsZero() || s.now().Sub(state.LastSentAt) >= cooldown
}

func (s Service) send(subject, body string) {
	if s.notifier == nil {
		log.Printf("monitor alert notification skipped subject=%q reason=no_notifier", subject)
		return
	}
	if err := s.notifier.Send(subject, body); err != nil {
		log.Printf("monitor alert notification failed subject=%q err=%v", subject, err)
	}
}

func (s Service) readState() alertState {
	path := strings.TrimSpace(s.cfg.MonitorAlertStatePath)
	if path == "" {
		return alertState{}
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return alertState{}
	}
	var state alertState
	if err := json.Unmarshal(body, &state); err != nil {
		log.Printf("monitor alert state decode failed path=%s err=%v", path, err)
		return alertState{}
	}
	return state
}

func (s Service) writeState(state alertState) {
	path := strings.TrimSpace(s.cfg.MonitorAlertStatePath)
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		log.Printf("monitor alert state mkdir failed path=%s err=%v", path, err)
		return
	}
	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		log.Printf("monitor alert state encode failed path=%s err=%v", path, err)
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(body, '\n'), 0o600); err != nil {
		log.Printf("monitor alert state write failed path=%s err=%v", path, err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		log.Printf("monitor alert state rename failed path=%s err=%v", path, err)
	}
}

type EmailNotifier struct {
	Enabled  bool
	Host     string
	Port     int
	Username string
	Password string
	From     string
	To       []string
}

func NewEmailNotifier(cfg config.Config) EmailNotifier {
	from := strings.TrimSpace(cfg.SMTPFrom)
	if from == "" {
		from = strings.TrimSpace(cfg.SMTPUsername)
	}
	return EmailNotifier{
		Enabled:  cfg.AlertEmailEnabled,
		Host:     strings.TrimSpace(cfg.SMTPHost),
		Port:     cfg.SMTPPort,
		Username: strings.TrimSpace(cfg.SMTPUsername),
		Password: cfg.SMTPPassword,
		From:     from,
		To:       cfg.SMTPTo,
	}
}

func (n EmailNotifier) Send(subject, body string) error {
	if !n.Enabled {
		log.Printf("monitor email disabled subject=%q", subject)
		return nil
	}
	if n.Host == "" || n.From == "" || len(n.To) == 0 {
		return errors.New("email alerting is enabled but SMTP_HOST, SMTP_FROM/SMTP_USERNAME, or SMTP_TO is missing")
	}
	port := n.Port
	if port <= 0 {
		port = 587
	}
	addr := n.Host + ":" + strconv.Itoa(port)
	var auth smtp.Auth
	if n.Username != "" || n.Password != "" {
		auth = smtp.PlainAuth("", n.Username, n.Password, n.Host)
	}
	message := strings.Join([]string{
		"From: " + n.From,
		"To: " + strings.Join(n.To, ", "),
		"Subject: " + sanitizeHeader(subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")
	if err := smtp.SendMail(addr, auth, n.From, n.To, []byte(message)); err != nil {
		return err
	}
	log.Printf("monitor email sent subject=%q recipients=%d", subject, len(n.To))
	return nil
}

func sanitizeHeader(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}
