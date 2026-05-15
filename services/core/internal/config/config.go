package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment                 string
	HTTPAddr                    string
	RiotAPIKey                  string
	ClickHouseHost              string
	ClickHousePort              int
	ClickHouseDatabase          string
	ClickHouseUser              string
	ClickHousePassword          string
	CORSOrigins                 []string
	DefaultPlatform             string
	RiotMinRequestInterval      time.Duration
	RiotRateLimitMaxRetries     int
	RiotRateLimitMaxSleep       time.Duration
	RiotAuthFailureExit         bool
	RiotAuthFailureMarkerPath   string
	CollectorPlatforms          []string
	CollectorDefaultMatchCount  int
	CollectorInterval           time.Duration
	CollectorIdleSleep          time.Duration
	CollectorFrontierBatchSize  int
	CollectorMaxRequests        int
	CollectorRateLimitRequests  int
	CollectorRateLimitWindow    time.Duration
	CollectorRateLimitReserve   int
	CollectorRecheckInterval    time.Duration
	CollectorDiscoveryDelay     time.Duration
	CollectorAutoSeedChallenger bool
	CollectorAutoSeedLimit      int
	RankEnrichmentEnabled       bool
	RankSnapshotTTL             time.Duration
	RankEnrichmentMaxRequests   int
}

func Load() Config {
	_ = godotenv.Load(".env", "../../.env", "../.env")
	defaultPlatform := env("DEFAULT_PLATFORM", "NA1")
	return Config{
		Environment:                 env("ENVIRONMENT", "development"),
		HTTPAddr:                    env("HTTP_ADDR", ":8000"),
		RiotAPIKey:                  os.Getenv("RIOT_API_KEY"),
		ClickHouseHost:              env("CLICKHOUSE_HOST", "localhost"),
		ClickHousePort:              envInt("CLICKHOUSE_PORT", 9000),
		ClickHouseDatabase:          env("CLICKHOUSE_DATABASE", "winrift"),
		ClickHouseUser:              env("CLICKHOUSE_USER", "winrift"),
		ClickHousePassword:          env("CLICKHOUSE_PASSWORD", "winrift"),
		CORSOrigins:                 splitOrigins(env("CORS_ORIGINS", "http://localhost:5173")),
		DefaultPlatform:             defaultPlatform,
		RiotMinRequestInterval:      time.Duration(envInt("RIOT_MIN_REQUEST_INTERVAL_MS", 75)) * time.Millisecond,
		RiotRateLimitMaxRetries:     envInt("RIOT_RATE_LIMIT_MAX_RETRIES", 3),
		RiotRateLimitMaxSleep:       time.Duration(envInt("RIOT_RATE_LIMIT_MAX_SLEEP_SECONDS", 120)) * time.Second,
		RiotAuthFailureExit:         envBool("RIOT_AUTH_FAILURE_EXIT", true),
		RiotAuthFailureMarkerPath:   env("RIOT_AUTH_FAILURE_MARKER_PATH", "/run/winrift/riot-auth-failed"),
		CollectorPlatforms:          splitList(env("COLLECTOR_PLATFORMS", defaultPlatform)),
		CollectorDefaultMatchCount:  envInt("COLLECTOR_DEFAULT_MATCH_COUNT", 20),
		CollectorInterval:           time.Duration(envInt("COLLECTOR_INTERVAL_SECONDS", 120)) * time.Second,
		CollectorIdleSleep:          time.Duration(envInt("COLLECTOR_IDLE_SLEEP_SECONDS", 15)) * time.Second,
		CollectorFrontierBatchSize:  envInt("COLLECTOR_FRONTIER_BATCH_SIZE", 3),
		CollectorMaxRequests:        envInt("COLLECTOR_MAX_REQUESTS_PER_PASS", 0),
		CollectorRateLimitRequests:  envInt("COLLECTOR_RATE_LIMIT_REQUESTS", 100),
		CollectorRateLimitWindow:    time.Duration(envInt("COLLECTOR_RATE_LIMIT_WINDOW_SECONDS", 120)) * time.Second,
		CollectorRateLimitReserve:   envInt("COLLECTOR_RATE_LIMIT_RESERVE_REQUESTS", 10),
		CollectorRecheckInterval:    time.Duration(envInt("COLLECTOR_RECHECK_HOURS", 24)) * time.Hour,
		CollectorDiscoveryDelay:     time.Duration(envInt("COLLECTOR_DISCOVERY_DELAY_MINUTES", 60)) * time.Minute,
		CollectorAutoSeedChallenger: envBool("COLLECTOR_AUTO_SEED_CHALLENGER", false),
		CollectorAutoSeedLimit:      envInt("COLLECTOR_AUTO_SEED_LIMIT_PER_PLATFORM", 3),
		RankEnrichmentEnabled:       envBool("RANK_ENRICHMENT_ENABLED", false),
		RankSnapshotTTL:             time.Duration(envInt("RANK_SNAPSHOT_TTL_HOURS", 24)) * time.Hour,
		RankEnrichmentMaxRequests:   envInt("RANK_ENRICHMENT_MAX_REQUESTS_PER_PASS", 5),
	}
}

func (c Config) IsDevelopment() bool {
	value := strings.ToLower(c.Environment)
	return value == "development" || value == "dev" || value == "local" || value == "test"
}

func (c Config) CollectorUsableRequestsPerRegion() int {
	requestLimit := c.CollectorRateLimitRequests
	if requestLimit <= 0 {
		requestLimit = 100
	}
	window := c.CollectorRateLimitWindow
	if window <= 0 {
		window = 120 * time.Second
	}
	interval := c.CollectorInterval
	if interval <= 0 {
		interval = window
	}
	budget := requestLimit
	if interval < window {
		budget = int((int64(requestLimit) * int64(interval)) / int64(window))
	}
	if budget < 1 {
		budget = 1
	}
	budget -= c.CollectorRateLimitReserve
	if budget < 1 {
		return 1
	}
	return budget
}

func (c Config) CollectorRankRequestBudget(totalRequests int) int {
	if !c.RankEnrichmentEnabled || c.RankEnrichmentMaxRequests <= 0 || totalRequests <= 1 {
		return 0
	}
	return min(c.RankEnrichmentMaxRequests, totalRequests-1)
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
	return splitList(value)
}

func splitList(value string) []string {
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
