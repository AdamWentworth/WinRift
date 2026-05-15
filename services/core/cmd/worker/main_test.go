package main

import (
	"errors"
	"net/http"
	"testing"

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
