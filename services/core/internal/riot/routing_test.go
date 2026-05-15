package riot

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"winrift/services/core/internal/config"
)

func TestRouting(t *testing.T) {
	if NormalizePlatform("na") != "NA1" {
		t.Fatal("NA alias failed")
	}
	region, err := RegionForPlatform("KR")
	if err != nil {
		t.Fatal(err)
	}
	if region != "ASIA" {
		t.Fatalf("region = %s", region)
	}
}

func TestParseRiotID(t *testing.T) {
	gameName, tagLine, err := ParseRiotID("Some Name#NA1")
	if err != nil {
		t.Fatal(err)
	}
	if gameName != "Some Name" || tagLine != "NA1" {
		t.Fatalf("got %q %q", gameName, tagLine)
	}
	if _, _, err := ParseRiotID("missing"); err == nil {
		t.Fatal("expected error")
	}
}

func TestIsAuthFailure(t *testing.T) {
	if !IsAuthFailure(APIError{StatusCode: http.StatusUnauthorized}) {
		t.Fatal("expected 401 to be auth failure")
	}
	if !IsAuthFailure(errors.Join(errors.New("wrapped"), APIError{StatusCode: http.StatusForbidden})) {
		t.Fatal("expected wrapped 403 to be auth failure")
	}
	if !IsAuthFailure(APIError{StatusCode: http.StatusServiceUnavailable, Message: "RIOT_API_KEY is not configured"}) {
		t.Fatal("expected missing Riot API key to be auth failure")
	}
	if IsAuthFailure(APIError{StatusCode: http.StatusTooManyRequests}) {
		t.Fatal("did not expect 429 to be auth failure")
	}
}

func TestRetryAfter(t *testing.T) {
	if got := retryAfter("3"); got != 3*time.Second {
		t.Fatalf("retryAfter = %s, want 3s", got)
	}
	if got := retryAfter(""); got != time.Second {
		t.Fatalf("empty retryAfter = %s, want 1s", got)
	}
}

func TestAuthFailureMarker(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "riot-auth-failed")
	client := &Client{authMarker: marker}
	client.markAuthFailure("NA1", "/lol/test/v1/resource", http.StatusForbidden)

	body, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "status=403") {
		t.Fatalf("marker body = %q, want status", body)
	}

	ClearAuthFailureMarker(config.Config{RiotAuthFailureMarkerPath: marker})
	if _, err := os.Stat(marker); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("marker still exists err=%v", err)
	}
}
