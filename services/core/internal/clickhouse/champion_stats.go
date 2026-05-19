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

func ChampionPerformanceKeyString(puuid string, championID, queueID uint16) string {
	return championPerformanceMapKey(puuid, championID, queueID)
}

func championPerformanceMapKey(puuid string, championID, queueID uint16) string {
	return fmt.Sprintf("%s:%d:%d", puuid, championID, queueID)
}
