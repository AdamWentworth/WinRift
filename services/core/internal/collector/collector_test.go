package collector

import (
	"context"
	"os"
	"testing"
	"time"

	"winrift/services/core/internal/analytics"
	"winrift/services/core/internal/riot"
)

type fakeRiot struct {
	matchIDs      []string
	matches       map[string][]byte
	match         []byte
	timeline      []byte
	timelineCalls int
	leagueEntries map[string][]riot.LeagueEntry
}

func (f *fakeRiot) MatchIDsByPUUID(context.Context, string, string, int) ([]string, error) {
	return f.matchIDs, nil
}

func (f *fakeRiot) MatchByID(_ context.Context, matchID, _ string) ([]byte, error) {
	if f.matches != nil {
		return f.matches[matchID], nil
	}
	return f.match, nil
}

func (f *fakeRiot) TimelineByMatchID(context.Context, string, string) ([]byte, error) {
	f.timelineCalls++
	return f.timeline, nil
}

func (f *fakeRiot) LeagueEntriesByPUUID(_ context.Context, puuid, _ string) ([]riot.LeagueEntry, error) {
	return f.leagueEntries[puuid], nil
}

type fakeRepo struct {
	insertedMatches  int
	frontierInserted int
	freshRanks       map[string]string
	rankSnapshots    []analytics.RankSnapshot
	lastNormalized   analytics.NormalizedMatch
}

func (f *fakeRepo) MatchExists(context.Context, string) (bool, error) {
	return false, nil
}

func (f *fakeRepo) InsertNormalized(_ context.Context, normalized analytics.NormalizedMatch) error {
	f.lastNormalized = normalized
	f.insertedMatches++
	return nil
}

func (f *fakeRepo) InsertFrontierParticipants(_ context.Context, participants []analytics.ParticipantRow, _ string, _ int16, _ time.Time) (int, error) {
	f.frontierInserted += len(participants)
	return len(participants), nil
}

func (f *fakeRepo) FreshRankBuckets(context.Context, string, []string, time.Time) (map[string]string, error) {
	if f.freshRanks == nil {
		return map[string]string{}, nil
	}
	return f.freshRanks, nil
}

func (f *fakeRepo) InsertRankSnapshot(_ context.Context, snapshot analytics.RankSnapshot) error {
	f.rankSnapshots = append(f.rankSnapshots, snapshot)
	return nil
}

func TestCollectAddsDiscoveredParticipantsToFrontier(t *testing.T) {
	raw, err := os.ReadFile("../../../../legacy/data/match_data.json")
	if err != nil {
		t.Fatal(err)
	}
	repo := &fakeRepo{}
	collector := New(&fakeRiot{
		matchIDs: []string{"NA1_1"},
		match:    raw,
		timeline: []byte(`{"info":{"frames":[]}}`),
	}, repo)

	result := collector.CollectFromPUUIDWithOptions(context.Background(), "seed-puuid", "NA1", CollectOptions{
		MatchCount:         1,
		MaxRequests:        3,
		DiscoveryDelay:     time.Hour,
		DiscoveredPriority: 0,
	})

	if result.MatchesInserted != 1 {
		t.Fatalf("matches inserted = %d, want 1", result.MatchesInserted)
	}
	if result.FrontierAdded != 10 || repo.frontierInserted != 10 {
		t.Fatalf("frontier added = %d repo = %d, want 10", result.FrontierAdded, repo.frontierInserted)
	}
	if result.RequestsUsed != 3 {
		t.Fatalf("requests used = %d, want 3", result.RequestsUsed)
	}
}

func TestCollectStopsAtRequestBudget(t *testing.T) {
	collector := New(&fakeRiot{matchIDs: []string{"NA1_1"}}, &fakeRepo{})

	result := collector.CollectFromPUUIDWithOptions(context.Background(), "seed-puuid", "NA1", CollectOptions{
		MatchCount:  1,
		MaxRequests: 1,
	})

	if !result.BudgetExhausted {
		t.Fatal("expected budget to be exhausted")
	}
	if result.MatchesInserted != 0 {
		t.Fatalf("matches inserted = %d, want 0", result.MatchesInserted)
	}
	if result.RequestsUsed != 1 {
		t.Fatalf("requests used = %d, want 1", result.RequestsUsed)
	}
}

func TestCollectEnrichesRankBucketFromLeagueEntries(t *testing.T) {
	raw, err := os.ReadFile("../../../../legacy/data/match_data.json")
	if err != nil {
		t.Fatal(err)
	}
	fixture, err := analytics.NormalizeMatch(raw, nil, "NA1", "UNKNOWN")
	if err != nil {
		t.Fatal(err)
	}
	targetPUUID := fixture.Participants[0].PUUID
	repo := &fakeRepo{}
	collector := New(&fakeRiot{
		matchIDs: []string{"NA1_1"},
		match:    raw,
		timeline: []byte(`{"info":{"frames":[]}}`),
		leagueEntries: map[string][]riot.LeagueEntry{
			targetPUUID: {
				{PUUID: targetPUUID, QueueType: "RANKED_SOLO_5x5", Tier: "GOLD", Rank: "II", LeaguePoints: 55, Wins: 20, Losses: 10},
			},
		},
	}, repo)

	result := collector.CollectFromPUUIDWithOptions(context.Background(), "seed-puuid", "NA1", CollectOptions{
		MatchCount:            1,
		MaxRequests:           3,
		RankEnrichmentEnabled: true,
		RankSnapshotTTL:       time.Hour,
		RankMaxRequests:       1,
		DiscoveryDelay:        time.Hour,
		DiscoveredPriority:    0,
	})

	if result.RankRequestsUsed != 1 {
		t.Fatalf("rank requests used = %d, want 1", result.RankRequestsUsed)
	}
	if result.RankSnapshotsInserted != 1 {
		t.Fatalf("rank snapshots inserted = %d, want 1", result.RankSnapshotsInserted)
	}
	if len(repo.rankSnapshots) != 1 || repo.rankSnapshots[0].RankBucket != "GOLD" {
		t.Fatalf("unexpected rank snapshots: %+v", repo.rankSnapshots)
	}
	for _, participant := range repo.lastNormalized.Participants {
		if participant.PUUID == targetPUUID && participant.RankBucket != "GOLD" {
			t.Fatalf("rank bucket = %q, want GOLD", participant.RankBucket)
		}
	}
}

func TestCollectStopsAtTargetPatchBoundaryBeforeTimeline(t *testing.T) {
	oldPatchMatch := []byte(`{
		"metadata": {"matchId": "NA1_old"},
		"info": {
			"gameId": 1,
			"gameVersion": "16.9.1.123",
			"queueId": 420,
			"mapId": 11,
			"gameMode": "CLASSIC"
		}
	}`)
	riotClient := &fakeRiot{
		matchIDs: []string{"NA1_old", "NA1_older"},
		matches:  map[string][]byte{"NA1_old": oldPatchMatch},
	}
	collector := New(riotClient, &fakeRepo{})

	result := collector.CollectFromPUUIDWithOptions(context.Background(), "seed-puuid", "NA1", CollectOptions{
		MatchCount:  2,
		MaxRequests: 10,
		TargetPatch: "16.10",
	})

	if !result.PatchBoundaryReached {
		t.Fatal("expected target patch boundary")
	}
	if result.MatchesInserted != 0 {
		t.Fatalf("matches inserted = %d, want 0", result.MatchesInserted)
	}
	if result.MatchesSkipped != 1 {
		t.Fatalf("matches skipped = %d, want 1", result.MatchesSkipped)
	}
	if result.RequestsUsed != 2 {
		t.Fatalf("requests used = %d, want 2", result.RequestsUsed)
	}
	if riotClient.timelineCalls != 0 {
		t.Fatalf("timeline calls = %d, want 0", riotClient.timelineCalls)
	}
}
