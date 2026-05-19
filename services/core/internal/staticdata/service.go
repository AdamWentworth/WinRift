package staticdata

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"
)

type RiotClient interface {
	DataDragonVersions(ctx context.Context) ([]string, error)
	DataDragonJSON(ctx context.Context, version, path string) (any, error)
}

type Service struct {
	riot          RiotClient
	mu            sync.Mutex
	latestVersion string
	cache         map[string]any
}

func NewService(riot RiotClient) *Service {
	return &Service{riot: riot, cache: map[string]any{}}
}

func (s *Service) Get(ctx context.Context, kind, patch string) (map[string]any, error) {
	fileByKind := map[string]string{
		"champions":       "champion.json",
		"items":           "item.json",
		"runes":           "runesReforged.json",
		"summoner-spells": "summoner.json",
	}
	file, ok := fileByKind[kind]
	if !ok {
		return nil, fmt.Errorf("unknown static data kind")
	}
	version := patch
	if version == "" {
		var err error
		version, err = s.LatestVersion(ctx)
		if err != nil {
			return nil, err
		}
	}
	key := version + ":" + kind
	s.mu.Lock()
	if data, ok := s.cache[key]; ok {
		s.mu.Unlock()
		return map[string]any{"version": version, "data": data}, nil
	}
	s.mu.Unlock()
	data, err := s.riot.DataDragonJSON(ctx, version, file)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cache[key] = data
	s.mu.Unlock()
	return map[string]any{"version": version, "data": data}, nil
}

func (s *Service) LatestVersion(ctx context.Context) (string, error) {
	s.mu.Lock()
	if s.latestVersion != "" {
		defer s.mu.Unlock()
		return s.latestVersion, nil
	}
	s.mu.Unlock()
	versions, err := s.riot.DataDragonVersions(ctx)
	if err != nil {
		return "", err
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("no Data Dragon versions returned")
	}
	s.mu.Lock()
	s.latestVersion = versions[0]
	s.mu.Unlock()
	return versions[0], nil
}

func (s *Service) BuildItemIDs(ctx context.Context, patch string, includeJungle bool) ([]uint32, error) {
	payload, err := s.Get(ctx, "items", patch)
	if err != nil {
		return nil, err
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected item data payload")
	}
	items, ok := data["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected item data shape")
	}
	ids := make([]uint32, 0, len(items))
	for rawID, rawItem := range items {
		id64, err := strconv.ParseUint(rawID, 10, 32)
		if err != nil {
			continue
		}
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		id := uint32(id64)
		if isBuildItem(id, item, includeJungle) {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})
	return ids, nil
}

func isBuildItem(id uint32, item map[string]any, includeJungle bool) bool {
	if excludedBuildItems[id] {
		return false
	}
	if consumed, _ := item["consumed"].(bool); consumed {
		return false
	}
	if maps, ok := item["maps"].(map[string]any); ok {
		if summonersRift, ok := maps["11"].(bool); ok && !summonersRift {
			return false
		}
	}
	tags := itemTags(item)
	if tags["Consumable"] || tags["Trinket"] {
		return false
	}
	if tags["Jungle"] {
		return includeJungle
	}
	totalGold, purchasable := itemGold(item)
	if !purchasable || totalGold < 700 {
		return false
	}
	depth := itemNumber(item["depth"])
	if tags["Boots"] {
		return depth >= 2 && totalGold >= 800
	}
	if depth >= 3 && totalGold >= 1500 {
		return true
	}
	if !hasItemTargets(item) && totalGold >= 1500 {
		return true
	}
	return false
}

func itemTags(item map[string]any) map[string]bool {
	out := map[string]bool{}
	rawTags, ok := item["tags"].([]any)
	if !ok {
		return out
	}
	for _, rawTag := range rawTags {
		tag, ok := rawTag.(string)
		if ok {
			out[tag] = true
		}
	}
	return out
}

func itemGold(item map[string]any) (int, bool) {
	gold, ok := item["gold"].(map[string]any)
	if !ok {
		return 0, false
	}
	purchasable, _ := gold["purchasable"].(bool)
	return itemNumber(gold["total"]), purchasable
}

func itemNumber(value any) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	case string:
		number, _ := strconv.Atoi(typed)
		return number
	default:
		return 0
	}
}

func hasItemTargets(item map[string]any) bool {
	targets, ok := item["into"].([]any)
	return ok && len(targets) > 0
}

var excludedBuildItems = map[uint32]bool{
	2052: true,
	2055: true,
	2138: true,
	2139: true,
	2140: true,
	3340: true,
	3348: true,
	3363: true,
	3364: true,
}
