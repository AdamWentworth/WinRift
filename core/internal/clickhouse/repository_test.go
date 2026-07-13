package clickhouse

import (
	"strings"
	"testing"

	"winrift/core/internal/config"
)

func TestClickHouseDSNIncludesOptionalMaintenanceSettings(t *testing.T) {
	dsn := clickHouseDSN(config.Config{
		ClickHouseHost:                    "clickhouse",
		ClickHousePort:                    9000,
		ClickHouseDatabase:                "winrift",
		ClickHouseUser:                    "winrift",
		ClickHousePassword:                "secret value",
		ClickHouseMaxThreads:              2,
		ClickHouseMaxMemoryMB:             512,
		ClickHouseMaxExecutionTimeSeconds: 1800,
	})

	wantParts := []string{
		"clickhouse://clickhouse:9000/winrift?",
		"username=winrift",
		"password=secret+value",
		"max_threads=2",
		"max_memory_usage=536870912",
		"max_execution_time=1800",
	}
	for _, part := range wantParts {
		if !strings.Contains(dsn, part) {
			t.Fatalf("dsn %q missing %q", dsn, part)
		}
	}
}

func TestPositiveOrDefault(t *testing.T) {
	if got := positiveOrDefault(3, 10); got != 3 {
		t.Fatalf("positive = %d, want 3", got)
	}
	if got := positiveOrDefault(0, 10); got != 10 {
		t.Fatalf("fallback = %d, want 10", got)
	}
}
