package staticdata

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
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

type ChampionSplash struct {
	ChampionID   string `json:"championId"`
	ChampionName string `json:"championName"`
	SkinName     string `json:"skinName"`
	SkinNumber   int    `json:"skinNumber"`
	Src          string `json:"src"`
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
	if kind == "champion-splashes" {
		data, err := s.ChampionSplashes(ctx, version)
		if err != nil {
			return nil, err
		}
		s.mu.Lock()
		s.cache[key] = data
		s.mu.Unlock()
		return map[string]any{"version": version, "data": data}, nil
	}
	file, ok := fileByKind[kind]
	if !ok {
		return nil, fmt.Errorf("unknown static data kind")
	}
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

func (s *Service) ChampionSplashes(ctx context.Context, version string) ([]ChampionSplash, error) {
	payload, err := s.Get(ctx, "champions", version)
	if err != nil {
		return nil, err
	}
	data, ok := payload["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected champion data payload")
	}
	rawChampions, ok := data["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected champion data shape")
	}
	champions := make([]ChampionSplash, 0, len(rawChampions))
	for _, rawChampion := range rawChampions {
		champion, ok := rawChampion.(map[string]any)
		if !ok {
			continue
		}
		id, _ := champion["id"].(string)
		name, _ := champion["name"].(string)
		if id == "" || name == "" {
			continue
		}
		champions = append(champions, ChampionSplash{ChampionID: id, ChampionName: name})
	}
	sort.Slice(champions, func(i, j int) bool {
		return champions[i].ChampionName < champions[j].ChampionName
	})

	results := make([][]ChampionSplash, len(champions))
	errs := make([]error, len(champions))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for index, champion := range champions {
		wg.Add(1)
		go func(index int, champion ChampionSplash) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			splashes, err := s.championSplashes(ctx, version, champion.ChampionID, champion.ChampionName)
			if err != nil {
				errs[index] = err
				return
			}
			results[index] = splashes
		}(index, champion)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}

	splashes := make([]ChampionSplash, 0, len(champions)*8)
	for _, championSplashes := range results {
		splashes = append(splashes, championSplashes...)
	}
	return splashes, nil
}

func (s *Service) championSplashes(ctx context.Context, version, championID, championName string) ([]ChampionSplash, error) {
	payload, err := s.riot.DataDragonJSON(ctx, version, fmt.Sprintf("champion/%s.json", championID))
	if err != nil {
		return nil, err
	}
	data, ok := payload.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected champion detail payload for %s", championID)
	}
	champions, ok := data["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected champion detail data for %s", championID)
	}
	champion, ok := champions[championID].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing champion detail for %s", championID)
	}
	rawSkins, ok := champion["skins"].([]any)
	if !ok {
		return nil, fmt.Errorf("missing skins for %s", championID)
	}
	splashes := make([]ChampionSplash, 0, len(rawSkins))
	for _, rawSkin := range rawSkins {
		skin, ok := rawSkin.(map[string]any)
		if !ok {
			continue
		}
		num, ok := numberAsInt(skin["num"])
		if !ok {
			continue
		}
		skinName, _ := skin["name"].(string)
		if skinName == "" || skinName == "default" {
			skinName = championName
		}
		if isChromaVariant(skinName) {
			continue
		}
		splashes = append(splashes, ChampionSplash{
			ChampionID:   championID,
			ChampionName: championName,
			SkinName:     skinName,
			SkinNumber:   num,
			Src:          fmt.Sprintf("https://ddragon.leagueoflegends.com/cdn/img/champion/splash/%s_%d.jpg", championID, num),
		})
	}
	sort.Slice(splashes, func(i, j int) bool {
		return splashes[i].SkinNumber < splashes[j].SkinNumber
	})
	return splashes, nil
}

func isChromaVariant(skinName string) bool {
	index := strings.LastIndex(skinName, " (")
	return index > 0 && strings.HasSuffix(skinName, ")")
}

func (s *Service) BuildItemIDs(ctx context.Context, patch string, includeJungle, includeSupport bool) ([]uint32, error) {
	return s.itemIDs(ctx, patch, func(id uint32, item map[string]any) bool {
		return isBuildItem(id, item, includeJungle, includeSupport)
	})
}

func (s *Service) StartingItemIDs(ctx context.Context, patch string, includeJungle, includeSupport bool) ([]uint32, error) {
	return s.itemIDs(ctx, patch, func(id uint32, item map[string]any) bool {
		return isStartingItem(id, item, includeJungle, includeSupport)
	})
}

func (s *Service) OpeningItemIDs(ctx context.Context, patch string, includeJungle, includeSupport bool) ([]uint32, error) {
	return s.itemIDs(ctx, patch, func(id uint32, item map[string]any) bool {
		return isOpeningItem(id, item, includeJungle, includeSupport)
	})
}

func (s *Service) OpeningItemCosts(ctx context.Context, patch string, includeJungle, includeSupport bool) (map[uint32]uint32, error) {
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
	costs := map[uint32]uint32{}
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
		if !isOpeningItem(id, item, includeJungle, includeSupport) {
			continue
		}
		totalGold, _ := itemGold(item)
		cost, ok := uint32FromInt(totalGold)
		if !ok {
			continue
		}
		costs[id] = cost
	}
	return costs, nil
}

func (s *Service) itemIDs(ctx context.Context, patch string, include func(uint32, map[string]any) bool) ([]uint32, error) {
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
		if include(id, item) {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		return ids[i] < ids[j]
	})
	return ids, nil
}

func numberAsInt(value any) (int, bool) {
	switch number := value.(type) {
	case float64:
		return int(number), true
	case int:
		return number, true
	default:
		return 0, false
	}
}

func isBuildItem(id uint32, item map[string]any, includeJungle, includeSupport bool) bool {
	if isExcludedShopItem(id, item) {
		return false
	}
	tags := itemTags(item)
	if tags["Consumable"] || tags["Trinket"] {
		return false
	}
	if tags["Jungle"] {
		return includeJungle
	}
	if isSupportBuildItem(tags) {
		return includeSupport
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

func isStartingItem(id uint32, item map[string]any, includeJungle, includeSupport bool) bool {
	if isExcludedShopItem(id, item) {
		return false
	}
	tags := itemTags(item)
	if tags["Consumable"] || tags["Trinket"] {
		return false
	}
	if tags["Jungle"] {
		return includeJungle
	}
	totalGold, purchasable := itemGold(item)
	if !purchasable || totalGold <= 0 || totalGold > 650 {
		return false
	}
	if isSupportBuildItem(tags) {
		return includeSupport
	}
	return true
}

func isOpeningItem(id uint32, item map[string]any, includeJungle, includeSupport bool) bool {
	if isUnavailableShopItem(item) {
		return false
	}
	tags := itemTags(item)
	if tags["Trinket"] {
		return false
	}
	if tags["Jungle"] {
		return includeJungle
	}
	if isSupportBuildItem(tags) {
		return includeSupport
	}
	totalGold, purchasable := itemGold(item)
	return purchasable && totalGold > 0 && totalGold <= 650
}

func isExcludedShopItem(id uint32, item map[string]any) bool {
	if excludedBuildItems[id] {
		return true
	}
	if consumed, _ := item["consumed"].(bool); consumed {
		return true
	}
	return isUnavailableShopItem(item)
}

func isUnavailableShopItem(item map[string]any) bool {
	if maps, ok := item["maps"].(map[string]any); ok {
		if summonersRift, ok := maps["11"].(bool); ok && !summonersRift {
			return true
		}
	}
	return false
}

func isSupportBuildItem(tags map[string]bool) bool {
	return tags["GoldPer"] && tags["Vision"] && tags["Lane"]
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

func uint32FromInt(value int) (uint32, bool) {
	parsed, err := strconv.ParseUint(strconv.Itoa(value), 10, 32)
	if err != nil {
		return 0, false
	}
	return uint32(parsed), true
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
