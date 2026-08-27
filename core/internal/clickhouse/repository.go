package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strconv"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"

	"winrift/core/internal/config"
)

type Repository struct {
	db          *sql.DB
	summaryOnly bool
}

const (
	clickHousePingTimeout   = 10 * time.Second
	clickHouseSchemaTimeout = 90 * time.Second
)

func NewRepository(cfg config.Config) (*Repository, error) {
	dsn := clickHouseDSN(cfg)
	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(positiveOrDefault(cfg.ClickHouseMaxOpenConns, 10))
	db.SetMaxIdleConns(positiveOrDefault(cfg.ClickHouseMaxIdleConns, 10))
	db.SetConnMaxLifetime(time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), clickHousePingTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	repo := &Repository{db: db, summaryOnly: !cfg.IsDevelopment()}
	schemaCtx, schemaCancel := context.WithTimeout(context.Background(), clickHouseSchemaTimeout)
	defer schemaCancel()
	if err := repo.EnsureRuntimeSchema(schemaCtx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return repo, nil
}

func clickHouseDSN(cfg config.Config) string {
	values := url.Values{}
	values.Set("username", cfg.ClickHouseUser)
	values.Set("password", cfg.ClickHousePassword)
	if cfg.ClickHouseMaxThreads > 0 {
		values.Set("max_threads", strconv.Itoa(cfg.ClickHouseMaxThreads))
	}
	if cfg.ClickHouseMaxMemoryMB > 0 {
		values.Set("max_memory_usage", strconv.FormatInt(int64(cfg.ClickHouseMaxMemoryMB)*1024*1024, 10))
	}
	if cfg.ClickHouseMaxExecutionTimeSeconds > 0 {
		values.Set("max_execution_time", strconv.Itoa(cfg.ClickHouseMaxExecutionTimeSeconds))
	}
	return fmt.Sprintf("clickhouse://%s:%d/%s?%s", cfg.ClickHouseHost, cfg.ClickHousePort, cfg.ClickHouseDatabase, values.Encode())
}

func positiveOrDefault(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

func (r *Repository) MatchExists(ctx context.Context, matchID string) (bool, error) {
	var count uint64
	err := r.db.QueryRowContext(ctx, "SELECT count() FROM raw_matches FINAL WHERE match_id = ?", matchID).Scan(&count)
	return count > 0, err
}
