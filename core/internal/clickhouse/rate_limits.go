package clickhouse

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

type RiotRequestReservation struct {
	Route     string
	Source    string
	Desired   int
	Granted   int
	Used      int
	Limit     int
	Available int
	Wait      time.Duration
}

func (r *Repository) ReserveRiotRequests(ctx context.Context, route, source string, desired, limit int, window time.Duration, now time.Time) (RiotRequestReservation, error) {
	reservation := RiotRequestReservation{
		Route:   normalizeRoute(route),
		Source:  strings.TrimSpace(source),
		Desired: desired,
		Limit:   limit,
	}
	if reservation.Route == "" || desired <= 0 {
		return reservation, nil
	}
	if reservation.Source == "" {
		reservation.Source = "unknown"
	}
	if reservation.Limit <= 0 {
		reservation.Limit = 100
	}
	if window <= 0 {
		window = 2 * time.Minute
	}
	if now.IsZero() {
		now = time.Now()
	}
	cutoff := now.Add(-window)
	used, err := r.riotRequestsUsed(ctx, reservation.Route, cutoff)
	if err != nil {
		return reservation, err
	}
	reservation.Used = used
	reservation.Available = reservation.Limit - used
	if reservation.Available < 0 {
		reservation.Available = 0
	}
	reservation.Granted = min(desired, reservation.Available)
	if reservation.Granted > 0 {
		if err := r.insertRiotRequestEvent(ctx, reservation.Route, reservation.Source, reservation.Granted, now); err != nil {
			return reservation, err
		}
		reservation.Available -= reservation.Granted
	}
	if reservation.Granted < desired {
		wait, err := r.riotRouteWait(ctx, reservation.Route, cutoff, window, now)
		if err != nil {
			return reservation, err
		}
		reservation.Wait = wait
	}
	return reservation, nil
}

func (r *Repository) riotRequestsUsed(ctx context.Context, route string, cutoff time.Time) (int, error) {
	var used uint64
	err := r.db.QueryRowContext(
		ctx,
		`SELECT coalesce(sum(request_count), 0)
		FROM riot_request_events
		WHERE route = ? AND happened_at > ?`,
		route,
		cutoff,
	).Scan(&used)
	return int(used), err
}

func (r *Repository) riotRouteWait(ctx context.Context, route string, cutoff time.Time, window time.Duration, now time.Time) (time.Duration, error) {
	var oldest time.Time
	err := r.db.QueryRowContext(
		ctx,
		`SELECT happened_at
		FROM riot_request_events
		WHERE route = ? AND happened_at > ?
		ORDER BY happened_at ASC
		LIMIT 1`,
		route,
		cutoff,
	).Scan(&oldest)
	if err != nil {
		if err == sql.ErrNoRows {
			return window, nil
		}
		return 0, err
	}
	wait := oldest.Add(window).Sub(now)
	if wait < 0 {
		return 0, nil
	}
	return wait, nil
}

func (r *Repository) insertRiotRequestEvent(ctx context.Context, route, source string, count int, happenedAt time.Time) error {
	if count <= 0 {
		return nil
	}
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO riot_request_events (route, source, request_count, happened_at) VALUES (?, ?, ?, ?)`,
		normalizeRoute(route),
		strings.TrimSpace(source),
		count,
		happenedAt,
	)
	return err
}

func normalizeRoute(route string) string {
	return strings.ToUpper(strings.TrimSpace(route))
}
