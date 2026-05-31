package collector

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"winrift/core/internal/analytics"
	"winrift/core/internal/riot"
)

type RiotClient interface {
	MatchIDsByPUUID(ctx context.Context, puuid, platform string, count int) ([]string, error)
	MatchByID(ctx context.Context, matchID, platform string) ([]byte, error)
	TimelineByMatchID(ctx context.Context, matchID, platform string) ([]byte, error)
	LeagueEntriesByPUUID(ctx context.Context, puuid, platform string) ([]riot.LeagueEntry, error)
}

type Repository interface {
	MatchExists(ctx context.Context, matchID string) (bool, error)
	InsertNormalized(ctx context.Context, normalized analytics.NormalizedMatch) error
	InsertFrontierParticipants(ctx context.Context, participants []analytics.ParticipantRow, sourceDetail string, priority int16, nextCheckAt time.Time) (int, error)
	FetchRankCandidates(ctx context.Context, platform string, limit int, now time.Time) ([]analytics.RankCandidate, error)
	FreshRankBuckets(ctx context.Context, platform string, puuids []string, now time.Time) (map[string]string, error)
	InsertRankSnapshot(ctx context.Context, snapshot analytics.RankSnapshot) error
}

type Collector struct {
	riot RiotClient
	repo Repository
}

type Result struct {
	MatchIDsSeen          int
	MatchesInserted       int
	MatchesSkipped        int
	FrontierAdded         int
	RequestsUsed          int
	RankRequestsUsed      int
	BudgetExhausted       bool
	RankBudgetExhausted   bool
	AuthFailed            bool
	RateLimited           bool
	PatchBoundaryReached  bool
	RankSnapshotsInserted int
	Errors                []string
}

type CollectOptions struct {
	MatchCount            int
	MaxRequests           int
	DiscoveryDelay        time.Duration
	DiscoveredPriority    int16
	ApplyCachedRanks      bool
	RankEnrichmentEnabled bool
	RankSnapshotTTL       time.Duration
	RankMaxRequests       int
	CurrentPatch          string
	PatchRetentionCount   int
}

type RankCollectOptions struct {
	MaxRequests     int
	CandidateLimit  int
	RankSnapshotTTL time.Duration
}

func New(riot RiotClient, repo Repository) Collector {
	return Collector{riot: riot, repo: repo}
}

func (c Collector) CollectFromPUUID(ctx context.Context, puuid, platform string, count int) Result {
	return c.CollectFromPUUIDWithOptions(ctx, puuid, platform, CollectOptions{MatchCount: count})
}

func (c Collector) CollectFromPUUIDWithOptions(ctx context.Context, puuid, platform string, options CollectOptions) Result {
	result := Result{}
	if options.MatchCount <= 0 {
		options.MatchCount = 1
	}
	normalizedPlatform := riot.NormalizePlatform(platform)
	patchWindow := analytics.PatchWindow(options.CurrentPatch, options.PatchRetentionCount)
	log.Printf(
		"collector start puuid=%s platform=%s match_count=%d max_requests=%d current_patch=%s patch_window=%s rank_enabled=%t rank_max_requests=%d",
		shortID(puuid),
		normalizedPlatform,
		options.MatchCount,
		options.MaxRequests,
		normalizedPatch(options.CurrentPatch),
		strings.Join(patchWindow, ","),
		options.RankEnrichmentEnabled,
		options.RankMaxRequests,
	)
	if !result.spendRequest(options.MaxRequests) {
		log.Printf("collector budget exhausted before match id lookup puuid=%s platform=%s", shortID(puuid), normalizedPlatform)
		return result
	}
	matchIDs, err := c.riot.MatchIDsByPUUID(ctx, puuid, platform, options.MatchCount)
	if err != nil {
		result.addRiotError(err)
		log.Printf("collector match ids failed puuid=%s platform=%s err=%v", shortID(puuid), normalizedPlatform, err)
		return result
	}
	result.MatchIDsSeen = len(matchIDs)
	log.Printf("collector match ids fetched puuid=%s platform=%s count=%d requests=%d", shortID(puuid), normalizedPlatform, len(matchIDs), result.RequestsUsed)
	for _, matchID := range matchIDs {
		exists, err := c.repo.MatchExists(ctx, matchID)
		if err != nil {
			result.Errors = append(result.Errors, matchID+": "+err.Error())
			log.Printf("collector match exists check failed match_id=%s err=%v", matchID, err)
			continue
		}
		if exists {
			result.MatchesSkipped++
			log.Printf("collector match skipped match_id=%s reason=already_ingested", matchID)
			continue
		}
		if !result.spendRequest(options.MaxRequests) {
			log.Printf("collector budget exhausted before match fetch match_id=%s requests=%d max_requests=%d", matchID, result.RequestsUsed, options.MaxRequests)
			break
		}
		log.Printf("collector match fetch start match_id=%s requests=%d", matchID, result.RequestsUsed)
		match, err := c.riot.MatchByID(ctx, matchID, platform)
		if err != nil {
			result.addRiotError(fmt.Errorf("%s: %w", matchID, err))
			log.Printf("collector match fetch failed match_id=%s err=%v", matchID, err)
			if result.shouldStop() {
				break
			}
			continue
		}
		if !analytics.ShouldIngest(match) {
			result.MatchesSkipped++
			log.Printf("collector match skipped match_id=%s reason=not_ranked_solo_summoners_rift", matchID)
			continue
		}
		if !patchPolicyAccepts(match, options.CurrentPatch, options.PatchRetentionCount) {
			patch, _ := analytics.MatchPatch(match)
			result.MatchesSkipped++
			result.PatchBoundaryReached = true
			log.Printf("collector match skipped match_id=%s reason=patch_retention_boundary patch=%s current_patch=%s retained_patches=%s action=move_to_next_puuid", matchID, patch, normalizedPatch(options.CurrentPatch), strings.Join(patchWindow, ","))
			break
		}
		if !result.spendRequest(options.MaxRequests) {
			log.Printf("collector budget exhausted before timeline fetch match_id=%s requests=%d max_requests=%d", matchID, result.RequestsUsed, options.MaxRequests)
			break
		}
		log.Printf("collector timeline fetch start match_id=%s requests=%d", matchID, result.RequestsUsed)
		timeline, err := c.riot.TimelineByMatchID(ctx, matchID, platform)
		if err != nil {
			result.addRiotError(fmt.Errorf("%s: %w", matchID, err))
			log.Printf("collector timeline fetch failed match_id=%s err=%v", matchID, err)
			if result.shouldStop() {
				break
			}
			continue
		}
		normalized, err := analytics.NormalizeMatch(match, timeline, normalizedPlatform, "UNKNOWN")
		if err != nil {
			result.Errors = append(result.Errors, matchID+": "+err.Error())
			log.Printf("collector normalize failed match_id=%s err=%v", matchID, err)
			continue
		}
		log.Printf(
			"collector match normalized match_id=%s patch=%s participants=%d item_events=%d frame_rows=%d",
			normalized.RawMatch.MatchID,
			normalized.RawMatch.Patch,
			len(normalized.Participants),
			len(normalized.TimelineItemEvents),
			len(normalized.TimelineParticipantFrames),
		)
		if options.RankEnrichmentEnabled {
			c.enrichRanks(ctx, &normalized, platform, options, &result)
		} else if options.ApplyCachedRanks {
			c.applyCachedRanks(ctx, &normalized, platform, options, &result)
		}
		if result.shouldStop() {
			log.Printf("collector stopping before insert match_id=%s auth_failed=%t rate_limited=%t", matchID, result.AuthFailed, result.RateLimited)
			break
		}
		if err := c.repo.InsertNormalized(ctx, normalized); err != nil {
			result.Errors = append(result.Errors, matchID+": "+err.Error())
			log.Printf("collector insert failed match_id=%s err=%v", matchID, err)
			continue
		}
		nextCheckAt := time.Now().Add(options.DiscoveryDelay)
		added, err := c.repo.InsertFrontierParticipants(ctx, normalized.Participants, normalized.RawMatch.MatchID, options.DiscoveredPriority, nextCheckAt)
		if err != nil {
			result.Errors = append(result.Errors, matchID+": frontier: "+err.Error())
			log.Printf("collector frontier insert failed match_id=%s err=%v", matchID, err)
		} else {
			result.FrontierAdded += added
		}
		result.MatchesInserted++
		log.Printf("collector match inserted match_id=%s frontier_added=%d total_inserted=%d", matchID, added, result.MatchesInserted)
	}
	log.Printf(
		"collector complete puuid=%s platform=%s seen=%d inserted=%d skipped=%d frontier_added=%d requests=%d rank_requests=%d rank_snapshots=%d errors=%d patch_boundary=%t budget_exhausted=%t rank_budget_exhausted=%t auth_failed=%t rate_limited=%t",
		shortID(puuid),
		normalizedPlatform,
		result.MatchIDsSeen,
		result.MatchesInserted,
		result.MatchesSkipped,
		result.FrontierAdded,
		result.RequestsUsed,
		result.RankRequestsUsed,
		result.RankSnapshotsInserted,
		len(result.Errors),
		result.PatchBoundaryReached,
		result.BudgetExhausted,
		result.RankBudgetExhausted,
		result.AuthFailed,
		result.RateLimited,
	)
	return result
}

func (c Collector) applyCachedRanks(ctx context.Context, normalized *analytics.NormalizedMatch, platform string, options CollectOptions, result *Result) {
	if options.RankSnapshotTTL <= 0 {
		options.RankSnapshotTTL = 24 * time.Hour
	}
	puuids := participantPUUIDs(normalized.Participants)
	now := time.Now()
	buckets, err := c.repo.FreshRankBuckets(ctx, riot.NormalizePlatform(platform), puuids, now)
	if err != nil {
		result.Errors = append(result.Errors, "rank cache: "+err.Error())
		log.Printf("collector cached rank lookup failed match_id=%s err=%v", normalized.RawMatch.MatchID, err)
		return
	}
	applyRankBuckets(normalized, buckets)
	log.Printf(
		"collector cached ranks applied match_id=%s participants=%d cache_hits=%d cache_misses=%d ttl_hours=%.1f",
		normalized.RawMatch.MatchID,
		len(puuids),
		len(buckets),
		len(puuids)-len(buckets),
		options.RankSnapshotTTL.Hours(),
	)
}

func (c Collector) enrichRanks(ctx context.Context, normalized *analytics.NormalizedMatch, platform string, options CollectOptions, result *Result) {
	if !options.RankEnrichmentEnabled {
		log.Printf("collector rank enrichment skipped match_id=%s reason=disabled", normalized.RawMatch.MatchID)
		return
	}
	if options.RankSnapshotTTL <= 0 {
		options.RankSnapshotTTL = 24 * time.Hour
	}
	puuids := participantPUUIDs(normalized.Participants)
	now := time.Now()
	buckets, err := c.repo.FreshRankBuckets(ctx, riot.NormalizePlatform(platform), puuids, now)
	if err != nil {
		result.Errors = append(result.Errors, "rank cache: "+err.Error())
		buckets = map[string]string{}
		log.Printf("collector rank cache failed match_id=%s err=%v", normalized.RawMatch.MatchID, err)
	}
	log.Printf(
		"collector rank enrichment start match_id=%s participants=%d cache_hits=%d cache_misses=%d ttl_hours=%.1f",
		normalized.RawMatch.MatchID,
		len(puuids),
		len(buckets),
		len(puuids)-len(buckets),
		options.RankSnapshotTTL.Hours(),
	)
	for _, puuid := range puuids {
		if buckets[puuid] != "" {
			continue
		}
		if !result.spendRankRequest(options.RankMaxRequests) {
			log.Printf("collector rank budget exhausted match_id=%s rank_requests=%d max_rank_requests=%d", normalized.RawMatch.MatchID, result.RankRequestsUsed, options.RankMaxRequests)
			break
		}
		log.Printf("collector rank fetch start match_id=%s puuid=%s rank_requests=%d", normalized.RawMatch.MatchID, shortID(puuid), result.RankRequestsUsed)
		entries, err := c.riot.LeagueEntriesByPUUID(ctx, puuid, platform)
		if err != nil {
			result.addRiotError(fmt.Errorf("rank %s: %w", puuid, err))
			log.Printf("collector rank fetch failed match_id=%s puuid=%s err=%v", normalized.RawMatch.MatchID, shortID(puuid), err)
			if result.shouldStop() {
				return
			}
			continue
		}
		snapshot := rankSnapshotFromEntries(puuid, riot.NormalizePlatform(platform), entries, now, now.Add(options.RankSnapshotTTL))
		if err := c.repo.InsertRankSnapshot(ctx, snapshot); err != nil {
			result.Errors = append(result.Errors, "rank snapshot: "+err.Error())
			log.Printf("collector rank snapshot insert failed match_id=%s puuid=%s err=%v", normalized.RawMatch.MatchID, shortID(puuid), err)
			continue
		}
		result.RankSnapshotsInserted++
		buckets[puuid] = snapshot.RankBucket
		log.Printf("collector rank snapshot stored match_id=%s puuid=%s bucket=%s tier=%s division=%s", normalized.RawMatch.MatchID, shortID(puuid), snapshot.RankBucket, snapshot.Tier, snapshot.Division)
	}
	applyRankBuckets(normalized, buckets)
	log.Printf("collector rank enrichment complete match_id=%s applied_buckets=%d rank_requests=%d rank_snapshots=%d", normalized.RawMatch.MatchID, len(buckets), result.RankRequestsUsed, result.RankSnapshotsInserted)
}

func (c Collector) CollectRanksForPlatform(ctx context.Context, platform string, options RankCollectOptions) Result {
	result := Result{}
	if options.MaxRequests <= 0 {
		return result
	}
	if options.RankSnapshotTTL <= 0 {
		options.RankSnapshotTTL = 24 * time.Hour
	}
	if options.CandidateLimit <= 0 || options.CandidateLimit > options.MaxRequests {
		options.CandidateLimit = options.MaxRequests
	}
	normalizedPlatform := riot.NormalizePlatform(platform)
	now := time.Now()
	candidates, err := c.repo.FetchRankCandidates(ctx, normalizedPlatform, options.CandidateLimit, now)
	if err != nil {
		result.Errors = append(result.Errors, "rank candidates: "+err.Error())
		log.Printf("rank lane candidates failed platform=%s err=%v", normalizedPlatform, err)
		return result
	}
	log.Printf(
		"rank lane start platform=%s candidates=%d max_requests=%d ttl_hours=%.1f",
		normalizedPlatform,
		len(candidates),
		options.MaxRequests,
		options.RankSnapshotTTL.Hours(),
	)
	for _, candidate := range candidates {
		if !result.spendRankRequest(options.MaxRequests) {
			log.Printf("rank lane budget exhausted platform=%s rank_requests=%d max_rank_requests=%d", normalizedPlatform, result.RankRequestsUsed, options.MaxRequests)
			break
		}
		log.Printf(
			"rank lane fetch start platform=%s puuid=%s participant_rows=%d unknown_rows=%d rank_requests=%d",
			normalizedPlatform,
			shortID(candidate.PUUID),
			candidate.ParticipantRows,
			candidate.UnknownRows,
			result.RankRequestsUsed,
		)
		entries, err := c.riot.LeagueEntriesByPUUID(ctx, candidate.PUUID, normalizedPlatform)
		if err != nil {
			result.addRiotError(fmt.Errorf("rank %s: %w", candidate.PUUID, err))
			log.Printf("rank lane fetch failed platform=%s puuid=%s err=%v", normalizedPlatform, shortID(candidate.PUUID), err)
			if result.shouldStop() {
				break
			}
			continue
		}
		snapshot := rankSnapshotFromEntries(candidate.PUUID, normalizedPlatform, entries, now, now.Add(options.RankSnapshotTTL))
		if err := c.repo.InsertRankSnapshot(ctx, snapshot); err != nil {
			result.Errors = append(result.Errors, "rank snapshot: "+err.Error())
			log.Printf("rank lane snapshot insert failed platform=%s puuid=%s err=%v", normalizedPlatform, shortID(candidate.PUUID), err)
			continue
		}
		result.RankSnapshotsInserted++
		log.Printf(
			"rank lane snapshot stored platform=%s puuid=%s bucket=%s tier=%s division=%s",
			normalizedPlatform,
			shortID(candidate.PUUID),
			snapshot.RankBucket,
			snapshot.Tier,
			snapshot.Division,
		)
	}
	log.Printf(
		"rank lane complete platform=%s candidates=%d rank_requests=%d rank_snapshots=%d errors=%d auth_failed=%t rate_limited=%t rank_budget_exhausted=%t",
		normalizedPlatform,
		len(candidates),
		result.RankRequestsUsed,
		result.RankSnapshotsInserted,
		len(result.Errors),
		result.AuthFailed,
		result.RateLimited,
		result.RankBudgetExhausted,
	)
	return result
}

func participantPUUIDs(participants []analytics.ParticipantRow) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(participants))
	for _, participant := range participants {
		if participant.PUUID == "" || seen[participant.PUUID] {
			continue
		}
		seen[participant.PUUID] = true
		out = append(out, participant.PUUID)
	}
	return out
}

func rankSnapshotFromEntries(puuid, platform string, entries []riot.LeagueEntry, fetchedAt, expiresAt time.Time) analytics.RankSnapshot {
	snapshot := analytics.RankSnapshot{
		PUUID:      puuid,
		Platform:   platform,
		QueueType:  "RANKED_SOLO_5x5",
		Tier:       "UNRANKED",
		Division:   "",
		RankBucket: "UNRANKED",
		FetchedAt:  fetchedAt,
		ExpiresAt:  expiresAt,
	}
	for _, entry := range entries {
		if strings.ToUpper(entry.QueueType) != "RANKED_SOLO_5X5" {
			continue
		}
		snapshot.QueueType = entry.QueueType
		snapshot.Tier = strings.ToUpper(entry.Tier)
		snapshot.Division = strings.ToUpper(entry.Rank)
		snapshot.LeaguePoints = entry.LeaguePoints
		snapshot.Wins = entry.Wins
		snapshot.Losses = entry.Losses
		snapshot.RankBucket = analytics.RankBucket(entry.Tier)
		return snapshot
	}
	return snapshot
}

func applyRankBuckets(normalized *analytics.NormalizedMatch, buckets map[string]string) {
	if len(buckets) == 0 {
		return
	}
	for i := range normalized.Participants {
		if bucket := buckets[normalized.Participants[i].PUUID]; bucket != "" {
			normalized.Participants[i].RankBucket = strings.ToUpper(bucket)
		}
	}
	normalized.Matchups = analytics.BuildMatchupRows(normalized.Participants)
}

func patchPolicyAccepts(match []byte, currentPatch string, retentionCount int) bool {
	current := normalizedPatch(currentPatch)
	if current == "" {
		return true
	}
	patch, ok := analytics.MatchPatch(match)
	return ok && analytics.PatchInWindow(patch, current, retentionCount)
}

func normalizedPatch(patch string) string {
	return strings.TrimSpace(patch)
}

func (r *Result) spendRequest(maxRequests int) bool {
	if maxRequests > 0 && r.RequestsUsed >= maxRequests {
		r.BudgetExhausted = true
		return false
	}
	r.RequestsUsed++
	return true
}

func (r *Result) spendRankRequest(maxRequests int) bool {
	if maxRequests > 0 && r.RankRequestsUsed >= maxRequests {
		r.RankBudgetExhausted = true
		return false
	}
	r.RankRequestsUsed++
	return true
}

func (r *Result) addRiotError(err error) {
	r.Errors = append(r.Errors, err.Error())
	var apiErr riot.APIError
	if !errors.As(err, &apiErr) {
		return
	}
	if riot.IsAuthFailure(err) {
		r.AuthFailed = true
		return
	}
	switch apiErr.StatusCode {
	case http.StatusTooManyRequests:
		r.RateLimited = true
	}
}

func (r Result) shouldStop() bool {
	return r.AuthFailed || r.RateLimited
}

func shortID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 12 {
		return value
	}
	return value[:8] + "..." + value[len(value)-4:]
}
