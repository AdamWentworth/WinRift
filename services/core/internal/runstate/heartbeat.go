package runstate

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type WorkerHeartbeat struct {
	Timestamp       time.Time `json:"timestamp"`
	Status          string    `json:"status"`
	Platforms       int       `json:"platforms,omitempty"`
	ActivePlatforms int       `json:"activePlatforms,omitempty"`
	Requests        int       `json:"requests,omitempty"`
	Message         string    `json:"message,omitempty"`
}

func WriteWorkerHeartbeat(path string, heartbeat WorkerHeartbeat) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if heartbeat.Timestamp.IsZero() {
		heartbeat.Timestamp = time.Now().UTC()
	}
	if heartbeat.Status == "" {
		heartbeat.Status = "active"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	body, err := json.MarshalIndent(heartbeat, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(body, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func ReadWorkerHeartbeat(path string) (WorkerHeartbeat, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return WorkerHeartbeat{}, errors.New("worker heartbeat path is empty")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return WorkerHeartbeat{}, err
	}
	var heartbeat WorkerHeartbeat
	if err := json.Unmarshal(body, &heartbeat); err != nil {
		return WorkerHeartbeat{}, err
	}
	return heartbeat, nil
}
