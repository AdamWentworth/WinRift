package clickhouse

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

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
