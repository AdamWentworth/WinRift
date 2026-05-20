package riot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"winrift/services/core/internal/config"
)

type Client struct {
	apiKey      string
	http        *http.Client
	minInterval time.Duration
	max429Retry int
	max429Sleep time.Duration
	authMarker  string
	throttleMu  sync.Mutex
	lastRequest time.Time
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e APIError) Error() string {
	return e.Message
}

func IsAuthFailure(err error) bool {
	var apiErr APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	return apiErr.StatusCode == http.StatusUnauthorized ||
		apiErr.StatusCode == http.StatusForbidden ||
		(apiErr.StatusCode == http.StatusServiceUnavailable && strings.Contains(apiErr.Message, "RIOT_API_KEY"))
}

type Account struct {
	PUUID    string `json:"puuid"`
	GameName string `json:"gameName"`
	TagLine  string `json:"tagLine"`
}

type Summoner struct {
	ID            string `json:"id"`
	AccountID     string `json:"accountId"`
	PUUID         string `json:"puuid"`
	ProfileIconID int    `json:"profileIconId"`
	SummonerLevel int64  `json:"summonerLevel"`
}

type LeagueList struct {
	LeagueID string        `json:"leagueId"`
	Tier     string        `json:"tier"`
	Queue    string        `json:"queue"`
	Name     string        `json:"name"`
	Entries  []LeagueEntry `json:"entries"`
}

type LeagueEntry struct {
	LeagueID     string `json:"leagueId"`
	SummonerID   string `json:"summonerId"`
	PUUID        string `json:"puuid"`
	QueueType    string `json:"queueType"`
	Tier         string `json:"tier"`
	Rank         string `json:"rank"`
	LeaguePoints int    `json:"leaguePoints"`
	Wins         int    `json:"wins"`
	Losses       int    `json:"losses"`
	HotStreak    bool   `json:"hotStreak"`
	Veteran      bool   `json:"veteran"`
	FreshBlood   bool   `json:"freshBlood"`
	Inactive     bool   `json:"inactive"`
}

func NewClient(cfg config.Config) *Client {
	return &Client{
		apiKey:      cfg.RiotAPIKey,
		http:        &http.Client{Timeout: 20 * time.Second},
		minInterval: cfg.RiotMinRequestInterval,
		max429Retry: cfg.RiotRateLimitMaxRetries,
		max429Sleep: cfg.RiotRateLimitMaxSleep,
		authMarker:  cfg.RiotAuthFailureMarkerPath,
	}
}

func (c *Client) AccountByRiotID(ctx context.Context, gameName, tagLine, platform string) (*Account, error) {
	region, err := AccountRegionForPlatform(platform)
	if err != nil {
		return nil, err
	}
	var account Account
	ok, err := c.get(ctx, region, fmt.Sprintf("/riot/account/v1/accounts/by-riot-id/%s/%s", escapePath(gameName), escapePath(tagLine)), nil, &account)
	if err != nil || !ok {
		return nil, err
	}
	return &account, nil
}

func (c *Client) AccountByPUUID(ctx context.Context, puuid, platform string) (*Account, error) {
	region, err := AccountRegionForPlatform(platform)
	if err != nil {
		return nil, err
	}
	var account Account
	ok, err := c.get(ctx, region, fmt.Sprintf("/riot/account/v1/accounts/by-puuid/%s", escapePath(puuid)), nil, &account)
	if err != nil || !ok {
		return nil, err
	}
	return &account, nil
}

func (c *Client) SummonerByPUUID(ctx context.Context, puuid, platform string) (*Summoner, error) {
	var summoner Summoner
	ok, err := c.get(ctx, NormalizePlatform(platform), fmt.Sprintf("/lol/summoner/v4/summoners/by-puuid/%s", escapePath(puuid)), nil, &summoner)
	if err != nil || !ok {
		return nil, err
	}
	return &summoner, nil
}

func (c *Client) SummonerByID(ctx context.Context, summonerID, platform string) (*Summoner, error) {
	var summoner Summoner
	ok, err := c.get(ctx, NormalizePlatform(platform), fmt.Sprintf("/lol/summoner/v4/summoners/%s", escapePath(summonerID)), nil, &summoner)
	if err != nil || !ok {
		return nil, err
	}
	return &summoner, nil
}

func (c *Client) ActiveGameByPUUID(ctx context.Context, puuid, platform string) (map[string]any, error) {
	var game map[string]any
	ok, err := c.get(ctx, NormalizePlatform(platform), fmt.Sprintf("/lol/spectator/v5/active-games/by-summoner/%s", escapePath(puuid)), nil, &game)
	if err != nil || !ok {
		return nil, err
	}
	return game, nil
}

func (c *Client) LeagueEntriesByPUUID(ctx context.Context, puuid, platform string) ([]LeagueEntry, error) {
	var entries []LeagueEntry
	_, err := c.get(ctx, NormalizePlatform(platform), fmt.Sprintf("/lol/league/v4/entries/by-puuid/%s", escapePath(puuid)), nil, &entries)
	return entries, err
}

func (c *Client) ChallengerLeagueByQueue(ctx context.Context, platform, queue string) (*LeagueList, error) {
	var league LeagueList
	ok, err := c.get(ctx, NormalizePlatform(platform), fmt.Sprintf("/lol/league/v4/challengerleagues/by-queue/%s", escapePath(queue)), nil, &league)
	if err != nil || !ok {
		return nil, err
	}
	return &league, nil
}

func (c *Client) MatchIDsByPUUID(ctx context.Context, puuid, platform string, count int) ([]string, error) {
	region, err := RegionForPlatform(platform)
	if err != nil {
		return nil, err
	}
	params := url.Values{}
	params.Set("start", "0")
	params.Set("count", strconv.Itoa(count))
	params.Set("queue", "420")
	var ids []string
	_, err = c.get(ctx, region, fmt.Sprintf("/lol/match/v5/matches/by-puuid/%s/ids", escapePath(puuid)), params, &ids)
	return ids, err
}

func (c *Client) MatchByID(ctx context.Context, matchID, platform string) ([]byte, error) {
	region, err := RegionForPlatform(platform)
	if err != nil {
		return nil, err
	}
	return c.getBytes(ctx, region, fmt.Sprintf("/lol/match/v5/matches/%s", escapePath(matchID)), nil, true)
}

func (c *Client) TimelineByMatchID(ctx context.Context, matchID, platform string) ([]byte, error) {
	region, err := RegionForPlatform(platform)
	if err != nil {
		return nil, err
	}
	return c.getBytes(ctx, region, fmt.Sprintf("/lol/match/v5/matches/%s/timeline", escapePath(matchID)), nil, true)
}

func (c *Client) DataDragonVersions(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://ddragon.leagueoflegends.com/api/versions.json", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, APIError{StatusCode: resp.StatusCode, Message: "Data Dragon versions request failed"}
	}
	var versions []string
	return versions, json.NewDecoder(resp.Body).Decode(&versions)
}

func (c *Client) DataDragonJSON(ctx context.Context, version, path string) (any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://ddragon.leagueoflegends.com/cdn/%s/data/en_US/%s", version, path), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, APIError{StatusCode: resp.StatusCode, Message: "Data Dragon request failed"}
	}
	var data any
	return data, json.NewDecoder(resp.Body).Decode(&data)
}

func (c *Client) get(ctx context.Context, route, path string, params url.Values, target any) (bool, error) {
	body, err := c.getBytes(ctx, route, path, params, true)
	if err != nil {
		var apiErr APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}
	return true, json.Unmarshal(body, target)
}

func (c *Client) getBytes(ctx context.Context, route, path string, params url.Values, retry429 bool) ([]byte, error) {
	retries := 0
	if retry429 {
		retries = c.max429Retry
	}
	return c.getBytesWithRetries(ctx, route, path, params, retries)
}

func (c *Client) getBytesWithRetries(ctx context.Context, route, path string, params url.Values, retriesLeft int) ([]byte, error) {
	if c.apiKey == "" {
		c.markAuthFailure(route, path, http.StatusServiceUnavailable)
		return nil, APIError{StatusCode: http.StatusServiceUnavailable, Message: "RIOT_API_KEY is not configured"}
	}
	if authFailureMarkerExists(c.authMarker) {
		return nil, APIError{StatusCode: http.StatusServiceUnavailable, Message: "Riot API key is unavailable; refresh the key and recreate the API/worker containers"}
	}
	if err := c.waitForTurn(ctx); err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("https://%s.api.riotgames.com%s", stringsLower(route), path)
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Riot-Token", c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	log.Printf("riot request route=%s path=%s status=%d", route, safeLogPath(path), resp.StatusCode)
	if resp.StatusCode == http.StatusTooManyRequests && retriesLeft > 0 {
		wait := retryAfter(resp.Header.Get("Retry-After"))
		if c.max429Sleep > 0 && wait > c.max429Sleep {
			log.Printf("riot rate limited route=%s path=%s retry_after=%s max_sleep=%s retries_left=%d action=defer_region", route, safeLogPath(path), wait, c.max429Sleep, retriesLeft)
			return nil, APIError{StatusCode: resp.StatusCode, Message: "Riot API rate limited"}
		}
		log.Printf("riot rate limited route=%s path=%s retry_after=%s retries_left=%d action=sleep_then_retry", route, safeLogPath(path), wait, retriesLeft)
		if err := sleepContext(ctx, wait); err != nil {
			return nil, err
		}
		return c.getBytesWithRetries(ctx, route, path, params, retriesLeft-1)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		c.markAuthFailure(route, path, resp.StatusCode)
		return nil, APIError{StatusCode: resp.StatusCode, Message: "Riot API key is missing, expired, or not authorized"}
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, APIError{StatusCode: resp.StatusCode, Message: "Riot resource not found"}
	}
	if resp.StatusCode >= 400 {
		return nil, APIError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("Riot API request failed with status %d", resp.StatusCode)}
	}
	return io.ReadAll(resp.Body)
}

func ClearAuthFailureMarker(cfg config.Config) {
	marker := strings.TrimSpace(cfg.RiotAuthFailureMarkerPath)
	if marker == "" {
		return
	}
	if err := os.Remove(marker); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("riot auth failure marker clear failed path=%s err=%v", marker, err)
	}
}

func AuthFailureMarkerExists(cfg config.Config) bool {
	return authFailureMarkerExists(cfg.RiotAuthFailureMarkerPath)
}

func StartAuthFailureMonitor(cfg config.Config, component string) {
	if !cfg.RiotAuthFailureExit {
		return
	}
	marker := strings.TrimSpace(cfg.RiotAuthFailureMarkerPath)
	if marker == "" {
		return
	}
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if _, err := os.Stat(marker); err == nil {
				log.Fatalf("%s stopping: Riot API auth failure marker exists path=%s", component, marker)
			} else if !errors.Is(err, os.ErrNotExist) {
				log.Printf("%s auth failure marker check failed path=%s err=%v", component, marker, err)
			}
		}
	}()
}

func (c *Client) markAuthFailure(route, apiPath string, status int) {
	marker := strings.TrimSpace(c.authMarker)
	if marker == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(marker), 0o700); err != nil {
		log.Printf("riot auth failure marker mkdir failed path=%s err=%v", marker, err)
		return
	}
	body := fmt.Sprintf("time=%s status=%d route=%s path=%s\n", time.Now().UTC().Format(time.RFC3339), status, route, safeLogPath(apiPath))
	if err := os.WriteFile(marker, []byte(body), 0o600); err != nil {
		log.Printf("riot auth failure marker write failed path=%s err=%v", marker, err)
		return
	}
	log.Printf("riot auth failure marker written path=%s status=%d route=%s path=%s", marker, status, route, safeLogPath(apiPath))
}

func authFailureMarkerExists(marker string) bool {
	marker = strings.TrimSpace(marker)
	if marker == "" {
		return false
	}
	_, err := os.Stat(marker)
	return err == nil
}

func retryAfter(value string) time.Duration {
	seconds, _ := strconv.Atoi(strings.TrimSpace(value))
	if seconds <= 0 {
		seconds = 1
	}
	return time.Duration(seconds) * time.Second
}

func sleepContext(ctx context.Context, wait time.Duration) error {
	if wait <= 0 {
		return nil
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *Client) waitForTurn(ctx context.Context) error {
	if c.minInterval <= 0 {
		return nil
	}
	c.throttleMu.Lock()
	now := time.Now()
	next := c.lastRequest.Add(c.minInterval)
	if next.Before(now) || next.Equal(now) {
		c.lastRequest = now
		c.throttleMu.Unlock()
		return nil
	}
	wait := time.Until(next)
	c.lastRequest = next
	c.throttleMu.Unlock()

	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func stringsLower(value string) string {
	out := make([]byte, len(value))
	for i := range value {
		ch := value[i]
		if ch >= 'A' && ch <= 'Z' {
			ch += 'a' - 'A'
		}
		out[i] = ch
	}
	return string(out)
}

func safeLogPath(path string) string {
	parts := strings.Split(path, "/")
	for i, part := range parts {
		if len(part) > 24 {
			parts[i] = part[:8] + "..." + part[len(part)-4:]
		}
	}
	return strings.Join(parts, "/")
}
