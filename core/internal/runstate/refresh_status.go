package runstate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type RefreshStatus struct {
	Name            string         `json:"name"`
	Patch           string         `json:"patch,omitempty"`
	QueueID         int            `json:"queueId,omitempty"`
	LastStartedAt   time.Time      `json:"lastStartedAt,omitempty"`
	LastSucceededAt time.Time      `json:"lastSucceededAt,omitempty"`
	LastFailedAt    time.Time      `json:"lastFailedAt,omitempty"`
	LastDurationMS  int64          `json:"lastDurationMs,omitempty"`
	Rows            map[string]int `json:"rows,omitempty"`
	LastError       string         `json:"lastError,omitempty"`
	Detail          string         `json:"detail,omitempty"`
}

type WorkerRefreshStatus struct {
	UpdatedAt time.Time       `json:"updatedAt"`
	Refreshes []RefreshStatus `json:"refreshes"`
}

func WriteWorkerRefreshStatus(path string, statuses map[string]RefreshStatus) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	keys := make([]string, 0, len(statuses))
	for key := range statuses {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := WorkerRefreshStatus{
		UpdatedAt: time.Now().UTC(),
		Refreshes: make([]RefreshStatus, 0, len(keys)),
	}
	for _, key := range keys {
		out.Refreshes = append(out.Refreshes, statuses[key])
	}
	body, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(body, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
