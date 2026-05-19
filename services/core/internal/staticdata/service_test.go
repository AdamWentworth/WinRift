package staticdata

import "testing"

func TestIsBuildItem(t *testing.T) {
	tests := []struct {
		name          string
		id            uint32
		item          map[string]any
		includeJungle bool
		want          bool
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
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := isBuildItem(test.id, test.item, test.includeJungle); got != test.want {
				t.Fatalf("isBuildItem() = %v, want %v", got, test.want)
			}
		})
	}
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
