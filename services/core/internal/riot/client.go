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
	"strconv"
	"time"

	"winrift/services/core/internal/config"
)

type Client struct {
	apiKey string
	http   *http.Client
}

type APIError struct {
	StatusCode int
	Message    string
}

func (e APIError) Error() string {
	return e.Message
}

type Account struct {
	PUUID    string `json:"puuid"`
	GameName string `json:"gameName"`
	TagLine  string `json:"tagLine"`
}

type Summoner struct {
	ID            string `json:"id"`
	AccountID     string `json:"accountId"`
	ProfileIconID int    `json:"profileIconId"`
	SummonerLevel int64  `json:"summonerLevel"`
}

func NewClient(cfg config.Config) *Client {
	return &Client{
		apiKey: cfg.RiotAPIKey,
		http:   &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Client) AccountByRiotID(ctx context.Context, gameName, tagLine, platform string) (*Account, error) {
	region, err := RegionForPlatform(platform)
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

func (c *Client) SummonerByPUUID(ctx context.Context, puuid, platform string) (*Summoner, error) {
	var summoner Summoner
	ok, err := c.get(ctx, NormalizePlatform(platform), fmt.Sprintf("/lol/summoner/v4/summoners/by-puuid/%s", escapePath(puuid)), nil, &summoner)
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
	if c.apiKey == "" {
		return nil, APIError{StatusCode: http.StatusServiceUnavailable, Message: "RIOT_API_KEY is not configured"}
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
	log.Printf("riot request route=%s path=%s status=%d", route, path, resp.StatusCode)
	if resp.StatusCode == http.StatusTooManyRequests && retry429 {
		retryAfter, _ := strconv.Atoi(resp.Header.Get("Retry-After"))
		if retryAfter <= 0 {
			retryAfter = 1
		}
		time.Sleep(time.Duration(retryAfter) * time.Second)
		return c.getBytes(ctx, route, path, params, false)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
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
