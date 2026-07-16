package clickhouse

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestRefreshSummonerIdentitySummaryProcessesPlatformsIndependently(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	compiledAt := time.Date(2026, time.July, 15, 23, 0, 0, 0, time.UTC)
	mock.ExpectQuery("SELECT DISTINCT platform FROM riot_account_aliases FINAL ORDER BY platform").
		WillReturnRows(sqlmock.NewRows([]string{"platform"}).AddRow("BR1").AddRow("KR"))
	platformQuery := "(?s)INSERT INTO summoner_identity_summary.*FROM riot_account_aliases FINAL.*WHERE platform = \\?.*FROM summoner_account_snapshots FINAL.*WHERE platform = \\?.*FROM summoner_identity_summary FINAL.*WHERE platform = \\?.*LEFT JOIN previous_identity.*SETTINGS"
	mock.ExpectExec(platformQuery).
		WithArgs("BR1", "BR1", "BR1", compiledAt).
		WillReturnResult(sqlmock.NewResult(0, 10))
	mock.ExpectExec(platformQuery).
		WithArgs("KR", "KR", "KR", compiledAt).
		WillReturnResult(sqlmock.NewResult(0, 10))

	repo := &Repository{db: db}
	if err := repo.refreshSummonerIdentitySummary(context.Background(), 420, compiledAt); err != nil {
		t.Fatalf("refresh summoner identity summary: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestSummonerIdentityRefreshDoesNotRescanRawMatches(t *testing.T) {
	sourceBytes, err := os.ReadFile("summoner_profile.go")
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	source := string(sourceBytes)
	start := strings.Index(source, "func (r *Repository) refreshSummonerIdentitySummary")
	if start < 0 {
		t.Fatal("could not locate identity refresh implementation")
	}
	end := strings.Index(source[start:], "func (r *Repository) refreshSummonerProfileTable")
	if end < 0 {
		t.Fatal("could not locate end of identity refresh implementation")
	}
	implementation := source[start : start+end]
	if strings.Contains(implementation, "FROM raw_matches") {
		t.Fatal("identity refresh must use durable identity data instead of rescanning raw matches")
	}
	if !strings.Contains(implementation, "FROM summoner_identity_summary FINAL") {
		t.Fatal("identity refresh does not preserve prior durable identity data")
	}
}

func TestRefreshSummonerProfileTableRetriesMemoryPressureInPlace(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	query := "INSERT INTO summoner_identity_summary SELECT ?"
	memoryPressure := errors.New("code: 241, memory limit exceeded by OvercommitTracker")
	mock.ExpectExec(query).WithArgs("identity").WillReturnError(memoryPressure)
	mock.ExpectExec(query).WithArgs("identity").WillReturnResult(sqlmock.NewResult(0, 1))

	repo := &Repository{db: db}
	if err := repo.refreshSummonerProfileTableWithDelay(
		context.Background(),
		"summoner_identity_summary",
		query,
		0,
		"identity",
	); err != nil {
		t.Fatalf("refresh summoner profile table: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestRefreshSummonerProfileTableReturnsPermanentError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	query := "INSERT INTO summoner_identity_summary SELECT ?"
	want := errors.New("unknown table")
	mock.ExpectExec(query).WithArgs("identity").WillReturnError(want)

	repo := &Repository{db: db}
	err = repo.refreshSummonerProfileTableWithDelay(
		context.Background(),
		"summoner_identity_summary",
		query,
		0,
		"identity",
	)
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestCleanupOldSummonerProfileSummariesRetriesMemoryPressure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	compiledAt := time.Date(2026, time.July, 15, 23, 0, 0, 0, time.UTC)
	identityCleanup := "ALTER TABLE summoner_identity_summary DELETE"
	mock.ExpectExec(identityCleanup).
		WithArgs(compiledAt).
		WillReturnError(errors.New("code: 341, mutation failed: Code: 241 memory limit exceeded"))
	mock.ExpectExec(identityCleanup).
		WithArgs(compiledAt).
		WillReturnResult(sqlmock.NewResult(0, 1))
	for _, table := range []string{
		"summoner_profile_summary",
		"summoner_champion_summary",
		"summoner_champion_role_summary",
		"summoner_recent_match_summary",
		"summoner_build_summary",
	} {
		mock.ExpectExec("ALTER TABLE "+table+" DELETE").
			WithArgs(uint16(420), compiledAt).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}

	repo := &Repository{db: db}
	if err := repo.cleanupOldSummonerProfileSummariesWithDelay(context.Background(), 420, compiledAt, 0); err != nil {
		t.Fatalf("cleanup summoner profile summaries: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestSummonerProfileRefreshUsesSpillableSettings(t *testing.T) {
	sourceBytes, err := os.ReadFile("summoner_profile.go")
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	source := string(sourceBytes)
	for _, setting := range []string{
		"join_algorithm = 'grace_hash'",
		"max_bytes_before_external_group_by = 268435456",
		"max_bytes_before_external_sort = 268435456",
	} {
		if count := strings.Count(source, setting); count < 6 {
			t.Fatalf("%q appears %d times, want at least 6", setting, count)
		}
	}
}
