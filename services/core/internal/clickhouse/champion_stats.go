package clickhouse

import (
	"context"
	"fmt"
	"strings"
)

type ChampionPerformanceKey struct {
	PUUID      string
	ChampionID uint16
	QueueID    uint16
}

type ChampionPerformance struct {
	PUUID      string
	Platform   string
	ChampionID uint16
	QueueID    uint16
	Role       string
	Games      int
	Wins       int
	Losses     int
	Kills      int
	Deaths     int
	Assists    int
	AvgKills   float64
	AvgDeaths  float64
	AvgAssists float64
	KDA        float64
	WinRate    float64
}

type ChampionRoleRate struct {
	ChampionID uint16
	Role       string
	Games      int
	TotalGames int
	PickRate   float64
}

func (r *Repository) ChampionPerformance(ctx context.Context, platform string, keys []ChampionPerformanceKey) (map[string]ChampionPerformance, error) {
	out := map[string]ChampionPerformance{}
	seen := map[string]bool{}
	for _, key := range keys {
		key.PUUID = strings.TrimSpace(key.PUUID)
		if key.PUUID == "" || key.ChampionID == 0 || key.QueueID == 0 {
			continue
		}
		mapKey := championPerformanceMapKey(key.PUUID, key.ChampionID, key.QueueID)
		if seen[mapKey] {
			continue
		}
		seen[mapKey] = true
		var games, wins, kills, deaths, assists uint64
		err := r.db.QueryRowContext(
			ctx,
			`SELECT
				count(),
				sum(win),
				sum(kills),
				sum(deaths),
				sum(assists)
			FROM participants FINAL
			WHERE platform = ? AND puuid = ? AND champion_id = ? AND queue_id = ?`,
			platform,
			key.PUUID,
			key.ChampionID,
			key.QueueID,
		).Scan(&games, &wins, &kills, &deaths, &assists)
		if err != nil {
			return nil, err
		}
		if games == 0 {
			continue
		}
		performance := ChampionPerformance{
			PUUID:      key.PUUID,
			Platform:   platform,
			ChampionID: key.ChampionID,
			QueueID:    key.QueueID,
			Games:      int(games),
			Wins:       int(wins),
			Losses:     int(games - wins),
			Kills:      int(kills),
			Deaths:     int(deaths),
			Assists:    int(assists),
			AvgKills:   float64(kills) / float64(games),
			AvgDeaths:  float64(deaths) / float64(games),
			AvgAssists: float64(assists) / float64(games),
			WinRate:    float64(wins) / float64(games),
		}
		if deaths > 0 {
			performance.KDA = float64(kills+assists) / float64(deaths)
		} else {
			performance.KDA = float64(kills + assists)
		}
		out[mapKey] = performance
	}
	return out, nil
}

func (r *Repository) ChampionRoleRates(ctx context.Context, championIDs []uint16, queueID uint16) ([]ChampionRoleRate, error) {
	if len(championIDs) == 0 {
		return nil, nil
	}
	ids := make([]uint32, 0, len(championIDs))
	seen := map[uint16]bool{}
	for _, championID := range championIDs {
		if championID == 0 || seen[championID] {
			continue
		}
		seen[championID] = true
		ids = append(ids, uint32(championID))
	}
	if len(ids) == 0 {
		return nil, nil
	}
	if queueID == 0 {
		queueID = 420
	}
	idList := uint32ListSQL(ids)
	rows, err := r.db.QueryContext(
		ctx,
		fmt.Sprintf(`
			WITH totals AS
			(
				SELECT
					champion_id,
					count() AS total_games
				FROM participants FINAL
				WHERE champion_id IN (%s)
					AND queue_id = ?
					AND role IN ('TOP', 'JUNGLE', 'MIDDLE', 'BOTTOM', 'UTILITY')
				GROUP BY champion_id
			)
			SELECT
				p.champion_id,
				p.role,
				count() AS games,
				t.total_games,
				toFloat64(games) / toFloat64(t.total_games) AS pick_rate
			FROM participants AS p FINAL
			INNER JOIN totals AS t ON p.champion_id = t.champion_id
			WHERE p.champion_id IN (%s)
				AND p.queue_id = ?
				AND p.role IN ('TOP', 'JUNGLE', 'MIDDLE', 'BOTTOM', 'UTILITY')
			GROUP BY p.champion_id, p.role, t.total_games
			ORDER BY p.champion_id ASC, games DESC`,
			idList,
			idList,
		),
		queueID,
		queueID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ChampionRoleRate
	for rows.Next() {
		var row ChampionRoleRate
		if err := rows.Scan(&row.ChampionID, &row.Role, &row.Games, &row.TotalGames, &row.PickRate); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func ChampionPerformanceKeyString(puuid string, championID, queueID uint16) string {
	return championPerformanceMapKey(puuid, championID, queueID)
}

func championPerformanceMapKey(puuid string, championID, queueID uint16) string {
	return fmt.Sprintf("%s:%d:%d", puuid, championID, queueID)
}
