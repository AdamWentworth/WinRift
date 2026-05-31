package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

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
