package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"winrift/services/core/internal/analytics"
	"winrift/services/core/internal/clickhouse"
	"winrift/services/core/internal/winconditions"
)

type winConditionRequest struct {
	BlueChampionIDs []uint16 `json:"blueChampionIds"`
	RedChampionIDs  []uint16 `json:"redChampionIds"`
	QueueID         uint16   `json:"queueId"`
	Patch           string   `json:"patch"`
	RankBucket      string   `json:"rankBucket"`
	MinGames        int      `json:"minGames"`
	MaxRows         int      `json:"maxRows"`
}

type winConditionResponse struct {
	CatalogPatch string                       `json:"catalogPatch"`
	Filters      winConditionFiltersResponse  `json:"filters"`
	Blue         winconditions.TeamProfile    `json:"blue"`
	Red          winconditions.TeamProfile    `json:"red"`
	BlueMatchups []winConditionMetricResponse `json:"blueMatchups"`
	RedMatchups  []winConditionMetricResponse `json:"redMatchups"`
}

type winConditionFiltersResponse struct {
	QueueID          uint16 `json:"queueId"`
	Patch            string `json:"patch"`
	RankBucket       string `json:"rankBucket"`
	RawTeamRows      int    `json:"rawTeamRows"`
	FilteredTeamRows int    `json:"filteredTeamRows"`
}

type winConditionMetricResponse struct {
	Condition         string                       `json:"condition"`
	Rating            string                       `json:"rating"`
	OpponentCondition string                       `json:"opponentCondition"`
	OpponentRating    string                       `json:"opponentRating"`
	Primary           bool                         `json:"primary"`
	Wins              int                          `json:"wins"`
	Games             int                          `json:"games"`
	WinRate           float64                      `json:"winRate"`
	Confidence        float64                      `json:"confidence"`
	MeetsMinGames     bool                         `json:"meetsMinGames"`
	Buckets           []winConditionBucketResponse `json:"buckets"`
}

type winConditionBucketResponse struct {
	Bucket        string  `json:"bucket"`
	Wins          int     `json:"wins"`
	Games         int     `json:"games"`
	WinRate       float64 `json:"winRate"`
	Confidence    float64 `json:"confidence"`
	MeetsMinGames bool    `json:"meetsMinGames"`
}

type winConditionAccumulator struct {
	wins    int
	games   int
	buckets map[string]*winConditionBucketAccumulator
}

type winConditionBucketAccumulator struct {
	wins  int
	games int
}

type historicalTeamProfile struct {
	row     clickhouse.TeamCompositionRow
	profile winconditions.TeamProfile
}

func (s Server) analyticsWinConditions(w http.ResponseWriter, r *http.Request) {
	var body winConditionRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if len(body.BlueChampionIDs) != 5 || len(body.RedChampionIDs) != 5 {
		writeError(w, http.StatusBadRequest, "expected exactly five blue champion ids and five red champion ids")
		return
	}
	queueID := body.QueueID
	if queueID == 0 {
		queueID = 420
	}
	minGames := body.MinGames
	if minGames <= 0 {
		minGames = 5
	}
	maxRows := body.MaxRows
	if maxRows <= 0 {
		maxRows = 200000
	}
	rankBucket := strings.ToUpper(strings.TrimSpace(body.RankBucket))
	if rankBucket == "ALL" {
		rankBucket = ""
	}

	blueProfile := s.winConds.TeamProfile(body.BlueChampionIDs)
	redProfile := s.winConds.TeamProfile(body.RedChampionIDs)
	rows, err := s.repo.QueryTeamCompositions(r.Context(), clickhouse.TeamCompositionFilters{
		QueueID: queueID,
		Patch:   strings.TrimSpace(body.Patch),
		MaxRows: maxRows,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	filteredRows := filterTeamRowsByRank(rows, rankBucket)
	blueMatchups := buildWinConditionMatchups(filteredRows, s.winConds, blueProfile, redProfile, minGames)
	redMatchups := buildWinConditionMatchups(filteredRows, s.winConds, redProfile, blueProfile, minGames)
	writeJSON(w, http.StatusOK, winConditionResponse{
		CatalogPatch: s.winConds.CatalogPatch(),
		Filters: winConditionFiltersResponse{
			QueueID:          queueID,
			Patch:            strings.TrimSpace(body.Patch),
			RankBucket:       rankBucket,
			RawTeamRows:      len(rows),
			FilteredTeamRows: len(filteredRows),
		},
		Blue:         blueProfile,
		Red:          redProfile,
		BlueMatchups: blueMatchups,
		RedMatchups:  redMatchups,
	})
}

func buildWinConditionMatchups(rows []clickhouse.TeamCompositionRow, analyzer winconditions.Analyzer, team winconditions.TeamProfile, opponent winconditions.TeamProfile, minGames int) []winConditionMetricResponse {
	matchTeams := historicalTeamsByMatch(rows, analyzer)
	out := make([]winConditionMetricResponse, 0, len(team.Axes))
	for _, axis := range team.Axes {
		primary := axis.Label == team.PrimaryCondition
		accumulator := aggregateWinConditionAxis(matchTeams, axis.Label, axis.Rating, opponent.PrimaryCondition, opponent.PrimaryRating, primary)
		out = append(out, accumulator.response(axis.Label, axis.Rating, opponent.PrimaryCondition, opponent.PrimaryRating, primary, minGames))
	}
	return out
}

func historicalTeamsByMatch(rows []clickhouse.TeamCompositionRow, analyzer winconditions.Analyzer) map[string][]historicalTeamProfile {
	out := make(map[string][]historicalTeamProfile)
	for _, row := range rows {
		out[row.MatchID] = append(out[row.MatchID], historicalTeamProfile{
			row:     row,
			profile: analyzer.TeamProfile(row.ChampionIDs),
		})
	}
	return out
}

func aggregateWinConditionAxis(matchTeams map[string][]historicalTeamProfile, condition, rating, opponentCondition, opponentRating string, primaryOnly bool) winConditionAccumulator {
	accumulator := newWinConditionAccumulator()
	for _, teams := range matchTeams {
		if len(teams) != 2 {
			continue
		}
		addWinConditionSample(&accumulator, teams[0], teams[1], condition, rating, opponentCondition, opponentRating, primaryOnly)
		addWinConditionSample(&accumulator, teams[1], teams[0], condition, rating, opponentCondition, opponentRating, primaryOnly)
	}
	return accumulator
}

func addWinConditionSample(accumulator *winConditionAccumulator, team historicalTeamProfile, opponent historicalTeamProfile, condition, rating, opponentCondition, opponentRating string, primaryOnly bool) {
	if !profileMatchesCondition(team.profile, condition, rating, primaryOnly) {
		return
	}
	if !profileMatchesCondition(opponent.profile, opponentCondition, opponentRating, true) {
		return
	}
	accumulator.add(team.row.Win, team.row.DurationSeconds)
}

func profileMatchesCondition(profile winconditions.TeamProfile, condition, rating string, primaryOnly bool) bool {
	if primaryOnly {
		return profile.PrimaryCondition == condition && profile.PrimaryRating == rating
	}
	return profile.Ratings[keyForWinCondition(condition)] == rating
}

func newWinConditionAccumulator() winConditionAccumulator {
	buckets := make(map[string]*winConditionBucketAccumulator, len(winConditionDurationBuckets))
	for _, bucket := range winConditionDurationBuckets {
		buckets[bucket] = &winConditionBucketAccumulator{}
	}
	return winConditionAccumulator{buckets: buckets}
}

func (a *winConditionAccumulator) add(win bool, durationSeconds uint32) {
	a.games++
	if win {
		a.wins++
	}
	bucket := a.buckets[gameLengthBucket(durationSeconds)]
	if bucket == nil {
		return
	}
	bucket.games++
	if win {
		bucket.wins++
	}
}

func (a winConditionAccumulator) response(condition, rating, opponentCondition, opponentRating string, primary bool, minGames int) winConditionMetricResponse {
	buckets := make([]winConditionBucketResponse, 0, len(winConditionDurationBuckets))
	for _, bucketName := range winConditionDurationBuckets {
		bucket := a.buckets[bucketName]
		buckets = append(buckets, winConditionBucketResponse{
			Bucket:        bucketName,
			Wins:          bucket.wins,
			Games:         bucket.games,
			WinRate:       winRatePercent(bucket.wins, bucket.games),
			Confidence:    round(analytics.WilsonLowerBound(bucket.wins, bucket.games, 1.96) * 100),
			MeetsMinGames: bucket.games >= minGames,
		})
	}
	return winConditionMetricResponse{
		Condition:         condition,
		Rating:            rating,
		OpponentCondition: opponentCondition,
		OpponentRating:    opponentRating,
		Primary:           primary,
		Wins:              a.wins,
		Games:             a.games,
		WinRate:           winRatePercent(a.wins, a.games),
		Confidence:        round(analytics.WilsonLowerBound(a.wins, a.games, 1.96) * 100),
		MeetsMinGames:     a.games >= minGames,
		Buckets:           buckets,
	}
}

func filterTeamRowsByRank(rows []clickhouse.TeamCompositionRow, rankBucket string) []clickhouse.TeamCompositionRow {
	if rankBucket == "" {
		return rows
	}
	out := make([]clickhouse.TeamCompositionRow, 0, len(rows))
	for _, row := range rows {
		if teamRankBucket(row.RankBuckets) == rankBucket {
			out = append(out, row)
		}
	}
	return out
}

func teamRankBucket(rankBuckets []string) string {
	counts := map[string]int{}
	for _, bucket := range rankBuckets {
		normalized := strings.ToUpper(strings.TrimSpace(bucket))
		if normalized == "" || normalized == "UNKNOWN" {
			continue
		}
		counts[normalized]++
	}
	if len(counts) == 0 {
		return "UNKNOWN"
	}
	bestBucket := "UNKNOWN"
	bestCount := -1
	bestRank := -1
	for bucket, count := range counts {
		rank := rankBucketOrder(bucket)
		if count > bestCount || (count == bestCount && rank > bestRank) {
			bestBucket = bucket
			bestCount = count
			bestRank = rank
		}
	}
	return bestBucket
}

func rankBucketOrder(bucket string) int {
	switch bucket {
	case "IRON":
		return 1
	case "BRONZE":
		return 2
	case "SILVER":
		return 3
	case "GOLD":
		return 4
	case "PLATINUM":
		return 5
	case "EMERALD":
		return 6
	case "DIAMOND":
		return 7
	case "MASTER":
		return 8
	case "GRANDMASTER":
		return 9
	case "CHALLENGER":
		return 10
	default:
		return 0
	}
}

func winRatePercent(wins, games int) float64 {
	if games <= 0 {
		return 0
	}
	return round(float64(wins) / float64(games) * 100)
}

func keyForWinCondition(condition string) string {
	switch condition {
	case "SplitPush":
		return "splitpush"
	case "Pick":
		return "pick"
	case "Siege":
		return "siege"
	case "Control":
		return "control"
	case "TeamFight":
		return "teamfight"
	default:
		return ""
	}
}

var winConditionDurationBuckets = []string{"0-20", "20-25", "25-30", "30-35", "35+"}

func gameLengthBucket(durationSeconds uint32) string {
	switch {
	case durationSeconds < 20*60:
		return "0-20"
	case durationSeconds < 25*60:
		return "20-25"
	case durationSeconds < 30*60:
		return "25-30"
	case durationSeconds < 35*60:
		return "30-35"
	default:
		return "35+"
	}
}
