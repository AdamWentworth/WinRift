package staticdata

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

func TestChampionSplashesIncludesSkinSplashArt(t *testing.T) {
	client := &fakeRiotClient{
		payloads: map[string]any{
			"champion.json": map[string]any{
				"data": map[string]any{
					"Aatrox": map[string]any{"id": "Aatrox", "name": "Aatrox"},
					"Ahri":   map[string]any{"id": "Ahri", "name": "Ahri"},
				},
			},
			"champion/Aatrox.json": championDetailFixture("Aatrox", []map[string]any{
				{"num": float64(0), "name": "default"},
				{"num": float64(7), "name": "Justicar Aatrox"},
				{"num": float64(8), "name": "Mecha Aatrox"},
				{"num": float64(9), "name": "Mecha Aatrox (Obsidian)"},
				{"num": float64(10), "name": "Pumpkin Prince Aatrox"},
				{"num": float64(11), "name": "Pumpkin Prince  Aatrox (Ruby)"},
			}),
			"champion/Ahri.json": championDetailFixture("Ahri", []map[string]any{
				{"num": float64(0), "name": "default"},
			}),
		},
		calls: map[string]int{},
	}
	service := NewService(client)

	payload, err := service.Get(context.Background(), "champion-splashes", "16.10.1")
	if err != nil {
		t.Fatalf("Get champion-splashes returned error: %v", err)
	}
	splashes, ok := payload["data"].([]ChampionSplash)
	if !ok {
		t.Fatalf("payload data type = %T, want []ChampionSplash", payload["data"])
	}
	if len(splashes) != 5 {
		t.Fatalf("len(splashes) = %d, want 5", len(splashes))
	}
	if splashes[0].ChampionName != "Aatrox" || splashes[0].SkinName != "Aatrox" || splashes[0].SkinNumber != 0 {
		t.Fatalf("first splash = %+v, want Aatrox default normalized to champion name", splashes[0])
	}
	if splashes[1].SkinName != "Justicar Aatrox" || splashes[1].Src != "https://ddragon.leagueoflegends.com/cdn/img/champion/splash/Aatrox_7.jpg" {
		t.Fatalf("second splash = %+v, want Justicar Aatrox splash URL", splashes[1])
	}
	for _, splash := range splashes {
		if splash.SkinName == "Mecha Aatrox (Obsidian)" || splash.SkinName == "Pumpkin Prince  Aatrox (Ruby)" {
			t.Fatalf("chroma variant should be skipped: %+v", splash)
		}
	}

	_, err = service.Get(context.Background(), "champion-splashes", "16.10.1")
	if err != nil {
		t.Fatalf("cached Get champion-splashes returned error: %v", err)
	}
	if got := client.callCount("champion/Aatrox.json"); got != 1 {
		t.Fatalf("Aatrox detail calls = %d, want cached single call", got)
	}
}

func TestIsBuildItem(t *testing.T) {
	tests := []struct {
		name           string
		id             uint32
		item           map[string]any
		includeJungle  bool
		includeSupport bool
		want           bool
	}{
		{
			name: "legendary item",
			id:   3071,
			item: itemFixture(3000, true, 3, []string{"Health", "Damage"}, nil),
			want: true,
		},
		{
			name: "upgraded boots",
			id:   3158,
			item: itemFixture(900, true, 2, []string{"Boots"}, []string{"3171"}),
			want: true,
		},
		{
			name: "starter item",
			id:   1055,
			item: itemFixture(450, true, 0, []string{"Health", "Damage", "Lane"}, nil),
			want: false,
		},
		{
			name: "component",
			id:   1038,
			item: itemFixture(1300, true, 0, []string{"Damage"}, []string{"3031"}),
			want: false,
		},
		{
			name: "control ward",
			id:   2055,
			item: itemFixture(75, true, 0, []string{"Consumable", "Vision"}, nil),
			want: false,
		},
		{
			name:          "jungle item for jungler",
			id:            1101,
			item:          itemFixture(450, true, 0, []string{"Jungle"}, nil),
			includeJungle: true,
			want:          true,
		},
		{
			name: "jungle item for non jungler",
			id:   1101,
			item: itemFixture(450, true, 0, []string{"Jungle"}, nil),
			want: false,
		},
		{
			name:           "support item for support context",
			id:             3870,
			item:           itemFixture(400, true, 2, []string{"Health", "ManaRegen", "Vision", "GoldPer", "Lane"}, nil),
			includeSupport: true,
			want:           true,
		},
		{
			name: "support item outside support context",
			id:   3870,
			item: itemFixture(400, true, 2, []string{"Health", "ManaRegen", "Vision", "GoldPer", "Lane"}, nil),
			want: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isBuildItem(test.id, test.item, test.includeJungle, test.includeSupport); got != test.want {
				t.Fatalf("isBuildItem() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestStartingItemIDs(t *testing.T) {
	client := &fakeRiotClient{
		payloads: map[string]any{
			"item.json": map[string]any{
				"data": map[string]any{
					"1001": itemFixture(300, true, 0, []string{"Boots"}, nil),
					"1036": itemFixture(350, true, 0, []string{"Damage"}, []string{"3071"}),
					"1055": itemFixture(450, true, 0, []string{"Health", "Damage", "Lane"}, nil),
					"1101": itemFixture(450, true, 0, []string{"Jungle"}, nil),
					"2003": itemFixture(50, true, 0, []string{"Consumable"}, nil),
					"2055": itemFixture(75, true, 0, []string{"Consumable", "Vision"}, nil),
					"3071": itemFixture(3000, true, 3, []string{"Health", "Damage"}, nil),
					"3870": itemFixture(400, true, 0, []string{"Health", "ManaRegen", "Vision", "GoldPer", "Lane"}, nil),
				},
			},
		},
		calls: map[string]int{},
	}
	service := NewService(client)

	defaultIDs, err := service.StartingItemIDs(context.Background(), "", false, false)
	if err != nil {
		t.Fatalf("StartingItemIDs default returned error: %v", err)
	}
	if got, want := defaultIDs, []uint32{1001, 1036, 1055}; !sameUint32s(got, want) {
		t.Fatalf("default starting IDs = %v, want %v", got, want)
	}

	jungleIDs, err := service.StartingItemIDs(context.Background(), "", true, false)
	if err != nil {
		t.Fatalf("StartingItemIDs jungle returned error: %v", err)
	}
	if got, want := jungleIDs, []uint32{1001, 1036, 1055, 1101}; !sameUint32s(got, want) {
		t.Fatalf("jungle starting IDs = %v, want %v", got, want)
	}

	supportIDs, err := service.StartingItemIDs(context.Background(), "", false, true)
	if err != nil {
		t.Fatalf("StartingItemIDs support returned error: %v", err)
	}
	if got, want := supportIDs, []uint32{1001, 1036, 1055, 3870}; !sameUint32s(got, want) {
		t.Fatalf("support starting IDs = %v, want %v", got, want)
	}
}

func TestOpeningItemIDsIncludesConsumables(t *testing.T) {
	client := &fakeRiotClient{
		payloads: map[string]any{
			"item.json": map[string]any{
				"data": map[string]any{
					"1001": itemFixture(300, true, 0, []string{"Boots"}, nil),
					"1056": itemFixture(400, true, 0, []string{"Mana", "Lane"}, nil),
					"1102": itemFixture(450, true, 0, []string{"Jungle"}, nil),
					"2003": itemFixture(50, true, 0, []string{"Consumable"}, nil),
					"2055": itemFixture(75, true, 0, []string{"Consumable", "Vision"}, nil),
					"3340": itemFixture(0, true, 0, []string{"Trinket"}, nil),
					"3071": itemFixture(3000, true, 3, []string{"Health", "Damage"}, nil),
					"3870": itemFixture(400, true, 0, []string{"Health", "ManaRegen", "Vision", "GoldPer", "Lane"}, nil),
				},
			},
		},
		calls: map[string]int{},
	}
	service := NewService(client)

	defaultIDs, err := service.OpeningItemIDs(context.Background(), "", false, false)
	if err != nil {
		t.Fatalf("OpeningItemIDs default returned error: %v", err)
	}
	if got, want := defaultIDs, []uint32{1001, 1056, 2003, 2055}; !sameUint32s(got, want) {
		t.Fatalf("default opening IDs = %v, want %v", got, want)
	}

	jungleIDs, err := service.OpeningItemIDs(context.Background(), "", true, false)
	if err != nil {
		t.Fatalf("OpeningItemIDs jungle returned error: %v", err)
	}
	if got, want := jungleIDs, []uint32{1001, 1056, 1102, 2003, 2055}; !sameUint32s(got, want) {
		t.Fatalf("jungle opening IDs = %v, want %v", got, want)
	}

	supportIDs, err := service.OpeningItemIDs(context.Background(), "", false, true)
	if err != nil {
		t.Fatalf("OpeningItemIDs support returned error: %v", err)
	}
	if got, want := supportIDs, []uint32{1001, 1056, 2003, 2055, 3870}; !sameUint32s(got, want) {
		t.Fatalf("support opening IDs = %v, want %v", got, want)
	}

	defaultCosts, err := service.OpeningItemCosts(context.Background(), "", false, false)
	if err != nil {
		t.Fatalf("OpeningItemCosts default returned error: %v", err)
	}
	if got, want := defaultCosts[1056], uint32(400); got != want {
		t.Fatalf("Doran opening cost = %d, want %d", got, want)
	}
	if got, want := defaultCosts[2003], uint32(50); got != want {
		t.Fatalf("potion opening cost = %d, want %d", got, want)
	}
	if _, ok := defaultCosts[1102]; ok {
		t.Fatalf("default opening costs should not include jungle item")
	}

	jungleCosts, err := service.OpeningItemCosts(context.Background(), "", true, false)
	if err != nil {
		t.Fatalf("OpeningItemCosts jungle returned error: %v", err)
	}
	if got, want := jungleCosts[1102], uint32(450); got != want {
		t.Fatalf("jungle opening cost = %d, want %d", got, want)
	}

	supportCosts, err := service.OpeningItemCosts(context.Background(), "", false, true)
	if err != nil {
		t.Fatalf("OpeningItemCosts support returned error: %v", err)
	}
	if got, want := supportCosts[3870], uint32(400); got != want {
		t.Fatalf("support opening cost = %d, want %d", got, want)
	}
}

func sameUint32s(left, right []uint32) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func itemFixture(totalGold int, purchasable bool, depth int, tags []string, into []string) map[string]any {
	item := map[string]any{
		"gold": map[string]any{
			"total":       float64(totalGold),
			"purchasable": purchasable,
		},
		"maps": map[string]any{
			"11": true,
		},
	}
	if depth > 0 {
		item["depth"] = float64(depth)
	}
	if len(tags) > 0 {
		rawTags := make([]any, 0, len(tags))
		for _, tag := range tags {
			rawTags = append(rawTags, tag)
		}
		item["tags"] = rawTags
	}
	if len(into) > 0 {
		rawInto := make([]any, 0, len(into))
		for _, target := range into {
			rawInto = append(rawInto, target)
		}
		item["into"] = rawInto
	}
	return item
}

type fakeRiotClient struct {
	mu       sync.Mutex
	payloads map[string]any
	calls    map[string]int
}

func (f *fakeRiotClient) DataDragonVersions(context.Context) ([]string, error) {
	return []string{"16.10.1"}, nil
}

func (f *fakeRiotClient) DataDragonJSON(_ context.Context, _ string, path string) (any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[path]++
	payload, ok := f.payloads[path]
	if !ok {
		return nil, fmt.Errorf("missing fixture for %s", path)
	}
	return payload, nil
}

func (f *fakeRiotClient) callCount(path string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[path]
}

func championDetailFixture(championID string, skins []map[string]any) map[string]any {
	rawSkins := make([]any, 0, len(skins))
	for _, skin := range skins {
		rawSkins = append(rawSkins, skin)
	}
	return map[string]any{
		"data": map[string]any{
			championID: map[string]any{
				"skins": rawSkins,
			},
		},
	}
}
