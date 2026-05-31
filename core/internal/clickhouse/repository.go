package clickhouse

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"

	"winrift/core/internal/config"
)

type Repository struct {
	db *sql.DB
}

const (
	clickHousePingTimeout   = 10 * time.Second
	clickHouseSchemaTimeout = 90 * time.Second
)

func NewRepository(cfg config.Config) (*Repository, error) {
	dsn := fmt.Sprintf("clickhouse://%s:%d/%s?username=%s&password=%s", cfg.ClickHouseHost, cfg.ClickHousePort, cfg.ClickHouseDatabase, cfg.ClickHouseUser, cfg.ClickHousePassword)
	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), clickHousePingTimeout)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, err
	}
	repo := &Repository{db: db}
	schemaCtx, schemaCancel := context.WithTimeout(context.Background(), clickHouseSchemaTimeout)
	defer schemaCancel()
	if err := repo.EnsureRuntimeSchema(schemaCtx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return repo, nil
}

func (r *Repository) MatchExists(ctx context.Context, matchID string) (bool, error) {
	var count uint64
	err := r.db.QueryRowContext(ctx, "SELECT count() FROM raw_matches FINAL WHERE match_id = ?", matchID).Scan(&count)
	return count > 0, err
}
