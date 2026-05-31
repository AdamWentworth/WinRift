package api

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"winrift/services/core/internal/riot"
)

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func (s Server) defaultPlatform(value string) string {
	if strings.TrimSpace(value) == "" {
		return s.cfg.DefaultPlatform
	}
	return riot.NormalizePlatform(value)
}

func (s Server) cors(next http.Handler) http.Handler {
	origins := map[string]bool{}
	for _, origin := range s.cfg.CORSOrigins {
		origins[origin] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *loggingResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *loggingResponseWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(body)
	w.bytes += n
	return n, err
}

func (s Server) logRequests(next http.Handler) http.Handler {
	if !s.cfg.APIRequestLogsEnabled {
		return next
	}
	slowThreshold := s.cfg.APISlowRequestThreshold
	if slowThreshold <= 0 {
		slowThreshold = 500 * time.Millisecond
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := &loggingResponseWriter{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}
		duration := time.Since(startedAt)
		route := r.Pattern
		if route == "" {
			route = r.URL.Path
		}
		slow := duration >= slowThreshold
		log.Printf(
			"api request method=%s route=%s status=%d duration_ms=%d bytes=%d cache=%s slow=%t",
			r.Method,
			route,
			status,
			duration.Milliseconds(),
			recorder.bytes,
			w.Header().Get("X-WinRift-Cache"),
			slow,
		)
	})
}

func writeRiotError(w http.ResponseWriter, err error) {
	if riot.IsAuthFailure(err) {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"code":   "RIOT_API_KEY_UNAVAILABLE",
			"detail": "Riot API key is unavailable. Refresh the key and recreate the API/worker containers.",
		})
		return
	}
	var apiErr riot.APIError
	if errors.As(err, &apiErr) {
		writeError(w, apiErr.StatusCode, apiErr.Message)
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}
func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case uint16:
		return int(typed)
	case uint64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil {
			return int(parsed)
		}
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return parsed
		}
	}
	return 0
}
func writeError(w http.ResponseWriter, status int, detail string) {
	writeJSON(w, status, map[string]string{"detail": detail})
}

type cachedAPIResponse struct {
	body      []byte
	expiresAt time.Time
}

type responseCache struct {
	mu      sync.Mutex
	entries map[string]cachedAPIResponse
}

func newResponseCache() *responseCache {
	return &responseCache{entries: map[string]cachedAPIResponse{}}
}

func (c *responseCache) get(key string) ([]byte, bool) {
	if c == nil || key == "" {
		return nil, false
	}
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[key]
	if !ok {
		return nil, false
	}
	if now.After(entry.expiresAt) {
		delete(c.entries, key)
		return nil, false
	}
	return append([]byte(nil), entry.body...), true
}

func (c *responseCache) set(key string, body []byte, ttl time.Duration) {
	if c == nil || key == "" || len(body) == 0 || ttl <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[key] = cachedAPIResponse{
		body:      append([]byte(nil), body...),
		expiresAt: time.Now().Add(ttl),
	}
}

func (s Server) writeCachedJSON(w http.ResponseWriter, status int, cacheKey string, ttl time.Duration, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encode response")
		return
	}
	if status >= 200 && status < 300 {
		s.responseCache.set(cacheKey, body, ttl)
	}
	writeJSONBytes(w, status, body, false)
}

func (s Server) writeCachedChampionPageJSON(w http.ResponseWriter, status int, cacheKey string, ttl time.Duration, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encode response")
		return
	}
	if status >= 200 && status < 300 {
		s.responseCache.set(cacheKey, body, ttl)
		if err := s.repo.StoreChampionPageBundle(context.Background(), cacheKey, body, ttl); err != nil {
			log.Printf("champion page persistent cache store failed key=%s err=%v", cacheKey, err)
		}
	}
	writeJSONBytes(w, status, body, false)
}

func writeJSONBytes(w http.ResponseWriter, status int, body []byte, cacheHit bool) {
	w.Header().Set("Content-Type", "application/json")
	if cacheHit {
		w.Header().Set("X-WinRift-Cache", "hit")
	} else {
		w.Header().Set("X-WinRift-Cache", "miss")
	}
	w.WriteHeader(status)
	_, _ = w.Write(append(body, '\n'))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func queryInt(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func queryBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func queryUint16List(value string) []uint16 {
	parts := strings.Split(value, ",")
	out := make([]uint16, 0, len(parts))
	seen := map[uint16]bool{}
	for _, part := range parts {
		parsed, err := strconv.ParseUint(strings.TrimSpace(part), 10, 16)
		if err != nil || parsed == 0 {
			continue
		}
		value := uint16(parsed)
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func uint16FromAny(value any) uint16 {
	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return uint16(typed)
		}
	case int64:
		if typed > 0 {
			return uint16(typed)
		}
	case uint16:
		return typed
	case uint64:
		if typed > 0 {
			return uint16(typed)
		}
	case float64:
		if typed > 0 {
			return uint16(typed)
		}
	case json.Number:
		parsed, err := typed.Int64()
		if err == nil && parsed > 0 {
			return uint16(parsed)
		}
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil && parsed > 0 {
			return uint16(parsed)
		}
	}
	return 0
}

func round(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}

func shortValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 12 {
		return value
	}
	return value[:8] + "..." + value[len(value)-4:]
}
