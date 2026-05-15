package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment                string
	HTTPAddr                   string
	RiotAPIKey                 string
	ClickHouseHost             string
	ClickHousePort             int
	ClickHouseDatabase         string
	ClickHouseUser             string
	ClickHousePassword         string
	CORSOrigins                []string
	DefaultPlatform            string
	CollectorDefaultMatchCount int
	CollectorInterval          time.Duration
	CollectorFrontierBatchSize int
	CollectorMaxRequests       int
	CollectorRecheckInterval   time.Duration
	CollectorDiscoveryDelay    time.Duration
	RankEnrichmentEnabled      bool
	RankSnapshotTTL            time.Duration
	RankEnrichmentMaxRequests  int
}

func Load() Config {
	_ = godotenv.Load(".env", "../../.env", "../.env")
	return Config{
		Environment:                env("ENVIRONMENT", "development"),
		HTTPAddr:                   env("HTTP_ADDR", ":8000"),
		RiotAPIKey:                 os.Getenv("RIOT_API_KEY"),
		ClickHouseHost:             env("CLICKHOUSE_HOST", "localhost"),
		ClickHousePort:             envInt("CLICKHOUSE_PORT", 9000),
		ClickHouseDatabase:         env("CLICKHOUSE_DATABASE", "winrift"),
		ClickHouseUser:             env("CLICKHOUSE_USER", "winrift"),
		ClickHousePassword:         env("CLICKHOUSE_PASSWORD", "winrift"),
		CORSOrigins:                splitOrigins(env("CORS_ORIGINS", "http://localhost:5173")),
		DefaultPlatform:            env("DEFAULT_PLATFORM", "NA1"),
		CollectorDefaultMatchCount: envInt("COLLECTOR_DEFAULT_MATCH_COUNT", 20),
		CollectorInterval:          time.Duration(envInt("COLLECTOR_INTERVAL_SECONDS", 3600)) * time.Second,
		CollectorFrontierBatchSize: envInt("COLLECTOR_FRONTIER_BATCH_SIZE", 3),
		CollectorMaxRequests:       envInt("COLLECTOR_MAX_REQUESTS_PER_PASS", 60),
		CollectorRecheckInterval:   time.Duration(envInt("COLLECTOR_RECHECK_HOURS", 24)) * time.Hour,
		CollectorDiscoveryDelay:    time.Duration(envInt("COLLECTOR_DISCOVERY_DELAY_MINUTES", 60)) * time.Minute,
		RankEnrichmentEnabled:      envBool("RANK_ENRICHMENT_ENABLED", false),
		RankSnapshotTTL:            time.Duration(envInt("RANK_SNAPSHOT_TTL_HOURS", 24)) * time.Hour,
		RankEnrichmentMaxRequests:  envInt("RANK_ENRICHMENT_MAX_REQUESTS_PER_PASS", 20),
	}
}

func (c Config) IsDevelopment() bool {
	value := strings.ToLower(c.Environment)
	return value == "development" || value == "dev" || value == "local" || value == "test"
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBool(key string, fallback bool) bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func splitOrigins(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
