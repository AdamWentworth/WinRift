package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
