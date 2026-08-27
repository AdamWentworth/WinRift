package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"winrift/core/internal/collector"
	"winrift/core/internal/config"
	"winrift/core/internal/riot"
	"winrift/core/internal/runstate"
)

var exitProcess = os.Exit

func isRiotAuthError(err error) bool {
	return riot.IsAuthFailure(err)
}

func isRiotRateLimitError(err error) bool {
	var apiErr riot.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusTooManyRequests
}

func stopForRiotAuthFailure(cfg config.Config, platformCount int) {
	writeWorkerHeartbeat(cfg, "auth_failed", platformCount, 0, 0, "Riot API key is missing, expired, or not authorized")
	log.Printf("collector stopping: Riot API key is missing, expired, or not authorized")
	// Authentication failures are intentional stops. Exiting successfully keeps
	// Docker's on-failure policy from retrying a key that needs operator action.
	exitProcess(0)
}

func writeWorkerHeartbeat(cfg config.Config, status string, platforms, activePlatforms, requests int, message string) {
	if err := runstate.WriteWorkerHeartbeat(cfg.MonitorWorkerHeartbeatPath, runstate.WorkerHeartbeat{
		Timestamp:       time.Now().UTC(),
		Status:          status,
		Platforms:       platforms,
		ActivePlatforms: activePlatforms,
		Requests:        requests,
		Message:         message,
	}); err != nil {
		log.Printf("collector heartbeat write failed path=%s err=%v", cfg.MonitorWorkerHeartbeatPath, err)
	}
}

func frontierStatus(result collector.Result) string {
	if result.AuthFailed {
		return "blocked"
	}
	if len(result.Errors) > 0 {
		return "error"
	}
	return "active"
}

func nextFrontierCheck(cfg config.Config, result collector.Result) time.Time {
	now := time.Now()
	if result.AuthFailed {
		return now.Add(24 * time.Hour)
	}
	if result.RateLimited || result.BudgetExhausted {
		return now.Add(cfg.CollectorInterval)
	}
	if len(result.Errors) > 0 {
		return now.Add(2 * cfg.CollectorInterval)
	}
	return now.Add(cfg.CollectorRecheckInterval)
}

func splitCSV(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func collectorPlatforms(cfg config.Config) []string {
	values := cfg.CollectorPlatforms
	if len(values) == 0 {
		values = []string{cfg.DefaultPlatform}
	}
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		platform := riot.NormalizePlatform(value)
		if platform == "" || seen[platform] {
			continue
		}
		if _, err := riot.RegionForPlatform(platform); err != nil {
			log.Printf("collector platform ignored platform=%s err=%v", platform, err)
			continue
		}
		seen[platform] = true
		out = append(out, platform)
	}
	if len(out) == 0 {
		out = []string{riot.NormalizePlatform(cfg.DefaultPlatform)}
	}
	return out
}

func shortValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 12 {
		return value
	}
	return value[:8] + "..." + value[len(value)-4:]
}
