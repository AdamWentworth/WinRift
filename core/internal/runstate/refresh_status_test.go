package runstate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteWorkerRefreshStatusSortsAndWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "refresh.json")
	statuses := map[string]RefreshStatus{
		"z-last": {
			Name:            "z-last",
			LastSucceededAt: time.Date(2026, 5, 31, 10, 0, 0, 0, time.UTC),
		},
		"a-first": {
			Name:   "a-first",
			Patch:  "16.11",
			Rows:   map[string]int{"contexts": 3},
			Detail: "test",
		},
	}

	if err := WriteWorkerRefreshStatus(path, statuses); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var written WorkerRefreshStatus
	if err := json.Unmarshal(body, &written); err != nil {
		t.Fatal(err)
	}
	if len(written.Refreshes) != 2 {
		t.Fatalf("refresh count = %d, want 2", len(written.Refreshes))
	}
	if written.Refreshes[0].Name != "a-first" || written.Refreshes[1].Name != "z-last" {
		t.Fatalf("refresh order = %q, %q", written.Refreshes[0].Name, written.Refreshes[1].Name)
	}
	if written.Refreshes[0].Rows["contexts"] != 3 {
		t.Fatalf("contexts = %d, want 3", written.Refreshes[0].Rows["contexts"])
	}
}
