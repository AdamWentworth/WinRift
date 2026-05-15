package main

import (
	"errors"
	"net/http"
	"testing"

	"winrift/services/core/internal/config"
	"winrift/services/core/internal/riot"
)

func TestIsRiotAuthError(t *testing.T) {
	if !isRiotAuthError(riot.APIError{StatusCode: http.StatusUnauthorized}) {
		t.Fatal("expected 401 to be an auth error")
	}
	if !isRiotAuthError(errors.Join(errors.New("wrapped"), riot.APIError{StatusCode: http.StatusForbidden})) {
		t.Fatal("expected wrapped 403 to be an auth error")
	}
	if isRiotAuthError(riot.APIError{StatusCode: http.StatusTooManyRequests}) {
		t.Fatal("did not expect 429 to be an auth error")
	}
}

func TestCollectorPlatformsNormalizesAndDedupes(t *testing.T) {
	platforms := collectorPlatforms(config.Config{
		DefaultPlatform:    "NA1",
		CollectorPlatforms: []string{"na", "EUW", "EUW1", "bad-platform", "kr"},
	})

	want := []string{"NA1", "EUW1", "KR"}
	if len(platforms) != len(want) {
		t.Fatalf("platform count = %d, want %d: %v", len(platforms), len(want), platforms)
	}
	for i := range want {
		if platforms[i] != want[i] {
			t.Fatalf("platform[%d] = %q, want %q; all=%v", i, platforms[i], want[i], platforms)
		}
	}
}
