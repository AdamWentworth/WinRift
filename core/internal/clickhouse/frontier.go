package clickhouse

import (
	"context"
	"time"

	"winrift/core/internal/analytics"
)

type FrontierSeed struct {
	PUUID        string
	Platform     string
	Source       string
	SourceDetail string
	Priority     int16
	NextCheckAt  time.Time
	Force        bool
}

type FrontierEntry struct {
	PUUID    string
	Platform string
	Source   string
	Priority int16
	Attempts uint32
}

func (r *Repository) InsertFrontierSeed(ctx context.Context, seed FrontierSeed) (bool, error) {
	if seed.PUUID == "" || seed.Platform == "" {
		return false, nil
	}
	if seed.Source == "" {
		seed.Source = "manual"
	}
	if seed.NextCheckAt.IsZero() {
		seed.NextCheckAt = time.Now()
	}
	var count uint64
	err := r.db.QueryRowContext(ctx, `SELECT count() FROM collector_frontier FINAL WHERE puuid = ? AND platform = ?`, seed.PUUID, seed.Platform).Scan(&count)
	if err != nil {
		return false, err
	}
	if count > 0 && !seed.Force {
		return false, nil
	}
	if count > 0 {
		_, err = r.db.ExecContext(
			ctx,
			`INSERT INTO collector_frontier
			(puuid, platform, source, source_detail, first_seen_at, last_checked_at, next_check_at, priority, matches_seen, matches_inserted, matches_skipped, errors, requests_used, attempts, status)
			SELECT
				puuid,
				platform,
				?,
				?,
				first_seen_at,
				last_checked_at,
				?,
				greatest(priority, ?),
				matches_seen,
				matches_inserted,
				matches_skipped,
				errors,
				requests_used,
				attempts,
				'pending'
			FROM collector_frontier FINAL
			WHERE puuid = ? AND platform = ?
			LIMIT 1`,
			seed.Source, seed.SourceDetail, seed.NextCheckAt, seed.Priority, seed.PUUID, seed.Platform,
		)
		return false, err
	}
	_, err = r.db.ExecContext(
		ctx,
		`INSERT INTO collector_frontier
		(puuid, platform, source, source_detail, first_seen_at, last_checked_at, next_check_at, priority, matches_seen, matches_inserted, matches_skipped, errors, requests_used, attempts, status)
		VALUES (?, ?, ?, ?, now(), NULL, ?, ?, 0, 0, 0, 0, 0, 0, 'pending')`,
		seed.PUUID, seed.Platform, seed.Source, seed.SourceDetail, seed.NextCheckAt, seed.Priority,
	)
	return err == nil, err
}

func (r *Repository) FetchDueFrontier(ctx context.Context, platform string, limit int) ([]FrontierEntry, error) {
	if limit <= 0 {
		limit = 1
	}
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT puuid, platform, source, priority, attempts
		FROM collector_frontier FINAL
		WHERE platform = ?
			AND status IN ('pending', 'active', 'error')
			AND next_check_at <= now()
		ORDER BY priority DESC, next_check_at ASC, attempts ASC
		LIMIT ?`,
		platform, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []FrontierEntry
	for rows.Next() {
		var entry FrontierEntry
		if err := rows.Scan(&entry.PUUID, &entry.Platform, &entry.Source, &entry.Priority, &entry.Attempts); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

func (r *Repository) MarkFrontierChecked(ctx context.Context, entry FrontierEntry, seen, inserted, skipped, errors, requestsUsed int, status string, nextCheckAt time.Time) error {
	if status == "" {
		status = "active"
	}
	if nextCheckAt.IsZero() {
		nextCheckAt = time.Now()
	}
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO collector_frontier
		(puuid, platform, source, source_detail, first_seen_at, last_checked_at, next_check_at, priority, matches_seen, matches_inserted, matches_skipped, errors, requests_used, attempts, status)
		SELECT
			puuid,
			platform,
			source,
			source_detail,
			first_seen_at,
			now(),
			?,
			priority,
			matches_seen + ?,
			matches_inserted + ?,
			matches_skipped + ?,
			errors + ?,
			requests_used + ?,
			attempts + 1,
			?
		FROM collector_frontier FINAL
		WHERE puuid = ? AND platform = ?
		LIMIT 1`,
		nextCheckAt, seen, inserted, skipped, errors, requestsUsed, status, entry.PUUID, entry.Platform,
	)
	return err
}

func (r *Repository) InsertFrontierParticipants(ctx context.Context, participants []analytics.ParticipantRow, sourceDetail string, priority int16, nextCheckAt time.Time) (int, error) {
	seen := map[string]bool{}
	inserted := 0
	for _, participant := range participants {
		key := participant.Platform + "\x00" + participant.PUUID
		if participant.PUUID == "" || participant.Platform == "" || seen[key] {
			continue
		}
		seen[key] = true
		ok, err := r.InsertFrontierSeed(ctx, FrontierSeed{
			PUUID:        participant.PUUID,
			Platform:     participant.Platform,
			Source:       "match-participant",
			SourceDetail: sourceDetail,
			Priority:     priority,
			NextCheckAt:  nextCheckAt,
		})
		if err != nil {
			return inserted, err
		}
		if ok {
			inserted++
		}
	}
	return inserted, nil
}
