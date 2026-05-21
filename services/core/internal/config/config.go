package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Environment                    string
	HTTPAddr                       string
	RiotAPIKey                     string
	ClickHouseHost                 string
	ClickHousePort                 int
	ClickHouseDatabase             string
	ClickHouseUser                 string
	ClickHousePassword             string
	CORSOrigins                    []string
	DefaultPlatform                string
	RiotMinRequestInterval         time.Duration
	RiotRateLimitMaxRetries        int
	RiotRateLimitMaxSleep          time.Duration
	RiotAuthFailureExit            bool
	RiotAuthFailureMarkerPath      string
	CollectorPlatforms             []string
	CollectorDefaultMatchCount     int
	CollectorCurrentPatch          string
	CollectorPatchRetention        int
	CollectorPruneOldPatches       bool
	CollectorInterval              time.Duration
	CollectorIdleSleep             time.Duration
	CollectorFrontierBatchSize     int
	CollectorMaxRequests           int
	CollectorRateLimitRequests     int
	CollectorRateLimitWindow       time.Duration
	CollectorRateLimitReserve      int
	CollectorRecheckInterval       time.Duration
	CollectorDiscoveryDelay        time.Duration
	CollectorAutoSeedChallenger    bool
	CollectorAutoSeedLimit         int
	RankEnrichmentEnabled          bool
	RankSnapshotTTL                time.Duration
	RankEnrichmentMaxRequests      int
	LiveRankEnrichmentEnabled      bool
	LiveRankMaxRequests            int
	LiveBackfillEnabled            bool
	LiveBackfillMinChampionGames   int
	LiveBackfillMaxSeeds           int
	LiveBackfillPriority           int
	LiveBackfillDelay              time.Duration
	AccountAliasEnrichmentEnabled  bool
	AccountAliasMaxRequests        int
	ItemSlotRefreshEnabled         bool
	ItemSlotRefreshInterval        time.Duration
	ChampionGuideRefreshEnabled    bool
	ChampionGuideRefreshInterval   time.Duration
	WinConditionRefreshEnabled     bool
	WinConditionRefreshInterval    time.Duration
	SummonerProfileRefreshEnabled  bool
	SummonerProfileRefreshInterval time.Duration
}

func Load() Config {
	_ = godotenv.Load(".env", "../../.env", "../.env")
	defaultPlatform := env("DEFAULT_PLATFORM", "NA1")
	return Config{
		Environment:                    env("ENVIRONMENT", "development"),
		HTTPAddr:                       env("HTTP_ADDR", ":8000"),
		RiotAPIKey:                     os.Getenv("RIOT_API_KEY"),
		ClickHouseHost:                 env("CLICKHOUSE_HOST", "localhost"),
		ClickHousePort:                 envInt("CLICKHOUSE_PORT", 9000),
		ClickHouseDatabase:             env("CLICKHOUSE_DATABASE", "winrift"),
		ClickHouseUser:                 env("CLICKHOUSE_USER", "winrift"),
		ClickHousePassword:             env("CLICKHOUSE_PASSWORD", "winrift"),
		CORSOrigins:                    splitOrigins(env("CORS_ORIGINS", "http://localhost:5173")),
		DefaultPlatform:                defaultPlatform,
		RiotMinRequestInterval:         time.Duration(envInt("RIOT_MIN_REQUEST_INTERVAL_MS", 75)) * time.Millisecond,
		RiotRateLimitMaxRetries:        envInt("RIOT_RATE_LIMIT_MAX_RETRIES", 3),
		RiotRateLimitMaxSleep:          time.Duration(envInt("RIOT_RATE_LIMIT_MAX_SLEEP_SECONDS", 120)) * time.Second,
		RiotAuthFailureExit:            envBool("RIOT_AUTH_FAILURE_EXIT", true),
		RiotAuthFailureMarkerPath:      env("RIOT_AUTH_FAILURE_MARKER_PATH", "/run/winrift/riot-auth-failed"),
		CollectorPlatforms:             splitList(env("COLLECTOR_PLATFORMS", defaultPlatform)),
		CollectorDefaultMatchCount:     envInt("COLLECTOR_DEFAULT_MATCH_COUNT", 20),
		CollectorCurrentPatch:          env("COLLECTOR_CURRENT_PATCH", env("COLLECTOR_TARGET_PATCH", "")),
		CollectorPatchRetention:        envInt("COLLECTOR_PATCH_RETENTION_COUNT", 2),
		CollectorPruneOldPatches:       envBool("COLLECTOR_PRUNE_OLD_PATCHES_ON_START", false),
		CollectorInterval:              time.Duration(envInt("COLLECTOR_INTERVAL_SECONDS", 120)) * time.Second,
		CollectorIdleSleep:             time.Duration(envInt("COLLECTOR_IDLE_SLEEP_SECONDS", 15)) * time.Second,
		CollectorFrontierBatchSize:     envInt("COLLECTOR_FRONTIER_BATCH_SIZE", 3),
		CollectorMaxRequests:           envInt("COLLECTOR_MAX_REQUESTS_PER_PASS", 0),
		CollectorRateLimitRequests:     envInt("COLLECTOR_RATE_LIMIT_REQUESTS", 100),
		CollectorRateLimitWindow:       time.Duration(envInt("COLLECTOR_RATE_LIMIT_WINDOW_SECONDS", 120)) * time.Second,
		CollectorRateLimitReserve:      envInt("COLLECTOR_RATE_LIMIT_RESERVE_REQUESTS", 10),
		CollectorRecheckInterval:       time.Duration(envInt("COLLECTOR_RECHECK_HOURS", 24)) * time.Hour,
		CollectorDiscoveryDelay:        time.Duration(envInt("COLLECTOR_DISCOVERY_DELAY_MINUTES", 60)) * time.Minute,
		CollectorAutoSeedChallenger:    envBool("COLLECTOR_AUTO_SEED_CHALLENGER", false),
		CollectorAutoSeedLimit:         envInt("COLLECTOR_AUTO_SEED_LIMIT_PER_PLATFORM", 3),
		RankEnrichmentEnabled:          envBool("RANK_ENRICHMENT_ENABLED", false),
		RankSnapshotTTL:                time.Duration(envInt("RANK_SNAPSHOT_TTL_HOURS", 24)) * time.Hour,
		RankEnrichmentMaxRequests:      envInt("RANK_ENRICHMENT_MAX_REQUESTS_PER_PASS", 5),
		LiveRankEnrichmentEnabled:      envBool("LIVE_RANK_ENRICHMENT_ENABLED", true),
		LiveRankMaxRequests:            envInt("LIVE_RANK_MAX_REQUESTS", 10),
		LiveBackfillEnabled:            envBool("LIVE_BACKFILL_ENABLED", true),
		LiveBackfillMinChampionGames:   envInt("LIVE_BACKFILL_MIN_CHAMPION_GAMES", 5),
		LiveBackfillMaxSeeds:           envInt("LIVE_BACKFILL_MAX_SEEDS", 10),
		LiveBackfillPriority:           envInt("LIVE_BACKFILL_PRIORITY", 95),
		LiveBackfillDelay:              time.Duration(envInt("LIVE_BACKFILL_DELAY_SECONDS", 0)) * time.Second,
		AccountAliasEnrichmentEnabled:  envBool("ACCOUNT_ALIAS_ENRICHMENT_ENABLED", true),
		AccountAliasMaxRequests:        envInt("ACCOUNT_ALIAS_MAX_REQUESTS_PER_PASS", 3),
		ItemSlotRefreshEnabled:         envBool("ITEM_SLOT_ANALYTICS_REFRESH_ENABLED", true),
		ItemSlotRefreshInterval:        time.Duration(envInt("ITEM_SLOT_ANALYTICS_REFRESH_INTERVAL_MINUTES", 10)) * time.Minute,
		ChampionGuideRefreshEnabled:    envBool("CHAMPION_GUIDE_ANALYTICS_REFRESH_ENABLED", true),
		ChampionGuideRefreshInterval:   time.Duration(envInt("CHAMPION_GUIDE_ANALYTICS_REFRESH_INTERVAL_MINUTES", 10)) * time.Minute,
		WinConditionRefreshEnabled:     envBool("WIN_CONDITION_ANALYTICS_REFRESH_ENABLED", true),
		WinConditionRefreshInterval:    time.Duration(envInt("WIN_CONDITION_ANALYTICS_REFRESH_INTERVAL_MINUTES", 15)) * time.Minute,
		SummonerProfileRefreshEnabled:  envBool("SUMMONER_PROFILE_ANALYTICS_REFRESH_ENABLED", true),
		SummonerProfileRefreshInterval: time.Duration(envInt("SUMMONER_PROFILE_ANALYTICS_REFRESH_INTERVAL_MINUTES", 10)) * time.Minute,
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

func (c Config) CollectorAccountAliasRequestBudget(remainingRequests int) int {
	if !c.AccountAliasEnrichmentEnabled || c.AccountAliasMaxRequests <= 0 || remainingRequests <= 1 {
		return 0
	}
	return min(c.AccountAliasMaxRequests, remainingRequests-1)
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
