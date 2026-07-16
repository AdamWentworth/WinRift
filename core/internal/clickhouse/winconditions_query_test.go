package clickhouse

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestQueryTeamCompositionsScopesJoinedInputs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	repo := &Repository{db: db}
	mock.ExpectQuery("(?s)WITH target_matches AS.*FROM raw_matches FINAL.*queue_id = \\?.*patch = \\?.*platform = \\?.*FROM participants AS p FINAL.*FROM summoner_rank_snapshots FINAL.*queue_type = 'RANKED_SOLO_5x5'.*platform = \\?.*p.queue_id = \\?.*p.patch = \\?.*p.platform = \\?.*p.match_id IN \\(SELECT match_id FROM target_matches\\).*INNER JOIN target_matches AS rm.*join_algorithm = 'grace_hash'.*max_bytes_before_external_group_by = 268435456").
		WithArgs(uint16(420), "16.12", "EUN1", "EUN1", uint16(420), "16.12", "EUN1").
		WillReturnRows(sqlmock.NewRows([]string{
			"match_id", "platform", "patch", "queue_id", "team_id", "win", "duration_seconds", "champion_ids", "rank_buckets",
		}))

	rows, err := repo.QueryTeamCompositions(context.Background(), TeamCompositionFilters{
		Patch:    "16.12",
		Platform: "eun1",
		QueueID:  420,
	})
	if err != nil {
		t.Fatalf("query team compositions: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("rows = %d, want 0", len(rows))
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
