package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"winrift/core/internal/analytics"
	"winrift/core/internal/clickhouse"
	"winrift/core/internal/config"
	"winrift/core/internal/riot"
)

func TestWriteRiotErrorMapsAuthFailureToUnavailable(t *testing.T) {
	recorder := httptest.NewRecorder()

	writeRiotError(recorder, riot.APIError{StatusCode: http.StatusForbidden, Message: "forbidden"})

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusServiceUnavailable)
	}
	var body map[string]string
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["code"] != "RIOT_API_KEY_UNAVAILABLE" {
		t.Fatalf("code = %q, want RIOT_API_KEY_UNAVAILABLE", body["code"])
	}
}

func TestWriteRiotErrorKeepsRateLimitStatus(t *testing.T) {
	recorder := httptest.NewRecorder()

	writeRiotError(recorder, riot.APIError{StatusCode: http.StatusTooManyRequests, Message: "Riot API rate limited"})

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusTooManyRequests)
	}
}

func TestLoggingResponseWriterCapturesFirstStatusAndBytes(t *testing.T) {
	recorder := httptest.NewRecorder()
	writer := &loggingResponseWriter{ResponseWriter: recorder}

	writer.WriteHeader(http.StatusCreated)
	writer.WriteHeader(http.StatusInternalServerError)
	_, _ = writer.Write([]byte("ok"))

	if writer.status != http.StatusCreated {
		t.Fatalf("captured status = %d, want %d", writer.status, http.StatusCreated)
	}
	if recorder.Code != http.StatusCreated {
		t.Fatalf("recorder status = %d, want %d", recorder.Code, http.StatusCreated)
	}
	if writer.bytes != 2 {
		t.Fatalf("bytes = %d, want 2", writer.bytes)
	}
}

func TestChampionPageBundleTTLUsesLongLivedArchivedCache(t *testing.T) {
	server := Server{cfg: config.Config{
		CollectorCurrentPatch:   "16.17",
		CollectorPatchRetention: 2,
	}}

	for _, patch := range []string{"16.17", "16.16"} {
		if got := server.championPageBundleTTL(patch); got != championPageBundleCurrentCacheTTL {
			t.Fatalf("TTL for retained patch %s = %s, want %s", patch, got, championPageBundleCurrentCacheTTL)
		}
	}
	if got := server.championPageBundleTTL("16.15"); got != championPageBundleArchivedCacheTTL {
		t.Fatalf("TTL for archived patch = %s, want %s", got, championPageBundleArchivedCacheTTL)
	}
}

func TestResponseFlightGroupCoalescesDuplicateWorkAndLetsWaitersCancel(t *testing.T) {
	group := newResponseFlightGroup()
	started := make(chan struct{})
	release := make(chan struct{})
	leaderDone := make(chan struct {
		body   []byte
		shared bool
		err    error
	}, 1)
	var calls atomic.Int32

	go func() {
		body, _, shared, err := group.do(context.Background(), "champion-page", func() ([]byte, bool, error) {
			calls.Add(1)
			close(started)
			<-release
			return []byte("bundle"), false, nil
		})
		leaderDone <- struct {
			body   []byte
			shared bool
			err    error
		}{body: body, shared: shared, err: err}
	}()

	<-started
	waiterCtx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, shared, err := group.do(waiterCtx, "champion-page", func() ([]byte, bool, error) {
		calls.Add(1)
		return []byte("duplicate"), false, nil
	})
	if err != context.Canceled {
		t.Fatalf("waiter error = %v, want context.Canceled", err)
	}
	if !shared {
		t.Fatal("duplicate waiter was not marked shared")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("build calls = %d, want 1", got)
	}

	close(release)
	select {
	case result := <-leaderDone:
		if result.err != nil {
			t.Fatal(result.err)
		}
		if result.shared {
			t.Fatal("leader was marked shared")
		}
		if string(result.body) != "bundle" {
			t.Fatalf("leader body = %q, want bundle", result.body)
		}
	case <-time.After(time.Second):
		t.Fatal("leader did not finish")
	}
}

func TestChampionPageBundleCacheKeyCanonicalizesEffectiveRequest(t *testing.T) {
	first, badRequest := parseChampionPageBundleRequest(url.Values{
		"championId":       {"86"},
		"role":             {" top "},
		"patch":            {"16.10"},
		"rankBucket":       {"diamond"},
		"minGames":         {"5"},
		"championMinGames": {"10"},
		"limit":            {"4"},
	})
	if badRequest != "" {
		t.Fatal(badRequest)
	}
	second, badRequest := parseChampionPageBundleRequest(url.Values{
		"limit":            {"4"},
		"championMinGames": {"10"},
		"minGames":         {"5"},
		"rankBucket":       {"DIAMOND"},
		"patch":            {"16.10"},
		"itemContext":      {"DEFAULT"},
		"role":             {"TOP"},
		"championId":       {"86"},
	})
	if badRequest != "" {
		t.Fatal(badRequest)
	}
	firstKey := championPageBundleCacheKey(first)
	secondKey := championPageBundleCacheKey(second)
	if firstKey != secondKey {
		t.Fatalf("cache keys differ:\nfirst:  %s\nsecond: %s", firstKey, secondKey)
	}

	noRank, badRequest := parseChampionPageBundleRequest(url.Values{
		"championId":       {"86"},
		"role":             {"TOP"},
		"patch":            {"16.10"},
		"minGames":         {"5"},
		"championMinGames": {"10"},
		"limit":            {"4"},
	})
	if badRequest != "" {
		t.Fatal(badRequest)
	}
	allRanks, badRequest := parseChampionPageBundleRequest(url.Values{
		"championId":       {"86"},
		"role":             {"TOP"},
		"patch":            {"16.10"},
		"rankBucket":       {"ALL"},
		"minGames":         {"5"},
		"championMinGames": {"10"},
		"limit":            {"4"},
	})
	if badRequest != "" {
		t.Fatal(badRequest)
	}
	if championPageBundleCacheKey(noRank) != championPageBundleCacheKey(allRanks) {
		t.Fatalf("cache key with rankBucket=ALL should match omitted all-ranks key")
	}
}

func TestChampionPageCanonicalAliasMatchesNoRoleFrontendRequest(t *testing.T) {
	resolved := championPagePrewarmRequest(62, "JUNGLE", "16.17", "", analytics.RankedSoloQueueID)
	alias := championPageCanonicalAliasRequest(resolved)
	if !championPageCanonicalAliasEligible(alias) {
		t.Fatalf("canonical alias should be eligible: %+v", alias.Build)
	}
	if alias.Build.Role != "" || alias.Build.ItemContext != "DEFAULT" || alias.Build.OpponentChampionID != 0 {
		t.Fatalf("canonical alias build = %+v", alias.Build)
	}
	if championPageBundleCacheKey(alias) == championPageBundleCacheKey(resolved) {
		t.Fatal("automatic-role alias must not overwrite the resolved-role cache key")
	}

	incoming, badRequest := parseChampionPageBundleRequest(url.Values{
		"championId":       {"62"},
		"patch":            {"16.17"},
		"minGames":         {"5"},
		"championMinGames": {"10"},
		"limit":            {"4"},
		"guideMinGames":    {"5"},
		"guideLimit":       {"12"},
		"indexMinGames":    {"1"},
		"indexLimit":       {"250"},
		"queueId":          {"420"},
	})
	if badRequest != "" {
		t.Fatal(badRequest)
	}
	if championPageBundleCacheKey(alias) != championPageBundleCacheKey(incoming) {
		t.Fatalf("alias key does not match no-role frontend request:\nalias:    %s\nincoming: %s", championPageBundleCacheKey(alias), championPageBundleCacheKey(incoming))
	}
}

func TestHydrateChampionPageRequestsLoadsResolvedAndAutomaticRoleKeysIntoMemory(t *testing.T) {
	server := Server{
		cfg: config.Config{
			CollectorCurrentPatch:   "16.17",
			CollectorPatchRetention: 2,
		},
		responseCache: newResponseCache(),
	}
	request := championPagePrewarmRequest(62, "JUNGLE", "16.17", "", analytics.RankedSoloQueueID)
	resolvedKey := championPageBundleCacheKey(request)
	aliasKey := championPageBundleCacheKey(championPageCanonicalAliasRequest(request))
	body := []byte(`{"champion":62}`)

	result, err := server.hydrateChampionPageRequests(
		context.Background(),
		[]championPageBundleRequest{request},
		func(_ context.Context, keys []string) (map[string]clickhouse.ChampionPageBundleCacheEntry, error) {
			if len(keys) != 1 || keys[0] != resolvedKey {
				t.Fatalf("hydration keys = %v, want %s", keys, resolvedKey)
			}
			return map[string]clickhouse.ChampionPageBundleCacheEntry{
				resolvedKey: {Body: body, ExpiresAt: time.Now().Add(time.Hour)},
			}, nil
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Candidates != 1 || result.Loaded != 1 || result.Missing != 0 {
		t.Fatalf("hydration result = %+v", result)
	}
	for _, key := range []string{resolvedKey, aliasKey} {
		cachedBody, ok := server.responseCache.get(key)
		if !ok || string(cachedBody) != string(body) {
			t.Fatalf("memory cache key %s ok=%t body=%s", key, ok, cachedBody)
		}
	}
}

func TestClearChampionPageBundleMemoryCacheKeepsUnrelatedResponses(t *testing.T) {
	server := Server{responseCache: newResponseCache()}
	server.responseCache.set(championPageBundleCacheKeyPrefix+"one", []byte(`{"page":1}`), time.Hour)
	server.responseCache.set("static:champions", []byte(`{"static":true}`), time.Hour)

	server.ClearChampionPageBundleMemoryCache()

	if _, ok := server.responseCache.get(championPageBundleCacheKeyPrefix + "one"); ok {
		t.Fatal("champion page cache entry was not cleared")
	}
	if body, ok := server.responseCache.get("static:champions"); !ok || string(body) != `{"static":true}` {
		t.Fatalf("unrelated cache entry ok=%t body=%s", ok, body)
	}
}
