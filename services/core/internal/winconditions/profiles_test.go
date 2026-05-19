package winconditions

import "testing"

func TestChampionProfilesAreValid(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if catalog.SchemaVersion != 1 {
		t.Fatalf("schema version = %d, want 1", catalog.SchemaVersion)
	}
	if catalog.Patch != "16.10.1" {
		t.Fatalf("patch = %q, want 16.10.1", catalog.Patch)
	}
	if len(catalog.Profiles) != 172 {
		t.Fatalf("profiles = %d, want 172", len(catalog.Profiles))
	}

	allowedWinConditions := map[string]bool{
		"SplitPush": true,
		"Pick":      true,
		"Siege":     true,
		"Control":   true,
		"TeamFight": true,
		"Flex":      true,
	}
	seenNames := map[string]bool{}
	seenIDs := map[uint16]bool{}
	blitzcrankCount := 0

	for _, profile := range catalog.Profiles {
		if profile.Champion == "" {
			t.Fatal("profile has empty champion name")
		}
		if profile.ChampionID == 0 {
			t.Fatalf("%s has empty champion id", profile.Champion)
		}
		if profile.Source == "" {
			t.Fatalf("%s has empty source", profile.Champion)
		}
		if seenNames[profile.Champion] {
			t.Fatalf("duplicate champion name %s", profile.Champion)
		}
		if seenIDs[profile.ChampionID] {
			t.Fatalf("duplicate champion id %d", profile.ChampionID)
		}
		seenNames[profile.Champion] = true
		seenIDs[profile.ChampionID] = true
		if profile.Champion == "Blitzcrank" {
			blitzcrankCount++
		}
		if !allowedWinConditions[profile.IndividualWinCondition] {
			t.Fatalf("%s has invalid individual win condition %q", profile.Champion, profile.IndividualWinCondition)
		}
		if profile.Scores.SplitPush < 0 || profile.Scores.Pick < 0 || profile.Scores.Siege < 0 || profile.Scores.Control < 0 || profile.Scores.TeamFight < 0 {
			t.Fatalf("%s has a negative score: %+v", profile.Champion, profile.Scores)
		}
		if total := profile.Scores.Total(); total != 10 {
			t.Fatalf("%s scores total %d, want 10: %+v", profile.Champion, total, profile.Scores)
		}
	}
	if blitzcrankCount != 1 {
		t.Fatalf("Blitzcrank count = %d, want 1", blitzcrankCount)
	}
}

func TestProposedChampionProfiles(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	profiles := map[string]Profile{}
	for _, profile := range catalog.Profiles {
		profiles[profile.Champion] = profile
	}

	expected := map[string]Profile{
		"Ambessa": {ChampionID: 799, IndividualWinCondition: "SplitPush", Scores: Scores{SplitPush: 4, Pick: 3, Siege: 0, Control: 1, TeamFight: 2}},
		"Aurora":  {ChampionID: 893, IndividualWinCondition: "Flex", Scores: Scores{SplitPush: 2, Pick: 3, Siege: 1, Control: 2, TeamFight: 2}},
		"Mel":     {ChampionID: 800, IndividualWinCondition: "Siege", Scores: Scores{SplitPush: 0, Pick: 2, Siege: 4, Control: 2, TeamFight: 2}},
		"Smolder": {ChampionID: 901, IndividualWinCondition: "TeamFight", Scores: Scores{SplitPush: 0, Pick: 1, Siege: 3, Control: 2, TeamFight: 4}},
		"Yunara":  {ChampionID: 804, IndividualWinCondition: "Flex", Scores: Scores{SplitPush: 1, Pick: 1, Siege: 3, Control: 2, TeamFight: 3}},
		"Zaahen":  {ChampionID: 904, IndividualWinCondition: "Flex", Scores: Scores{SplitPush: 3, Pick: 2, Siege: 0, Control: 2, TeamFight: 3}},
	}

	for champion, want := range expected {
		got, ok := profiles[champion]
		if !ok {
			t.Fatalf("%s profile missing", champion)
		}
		if got.Source != "proposed-16.10.1" {
			t.Fatalf("%s source = %q, want proposed-16.10.1", champion, got.Source)
		}
		if got.ChampionID != want.ChampionID || got.IndividualWinCondition != want.IndividualWinCondition || got.Scores != want.Scores {
			t.Fatalf("%s profile = %+v, want %+v", champion, got, want)
		}
	}
}

func TestTeamProfileScoresAndRatings(t *testing.T) {
	catalog := Catalog{
		Patch: "test",
		Profiles: []Profile{
			{Champion: "One", ChampionID: 1, IndividualWinCondition: "Pick", Scores: Scores{SplitPush: 1, Pick: 4, Siege: 1, Control: 1, TeamFight: 3}},
			{Champion: "Two", ChampionID: 2, IndividualWinCondition: "TeamFight", Scores: Scores{SplitPush: 1, Pick: 2, Siege: 1, Control: 2, TeamFight: 4}},
			{Champion: "Three", ChampionID: 3, IndividualWinCondition: "Pick", Scores: Scores{SplitPush: 1, Pick: 4, Siege: 0, Control: 2, TeamFight: 3}},
			{Champion: "Four", ChampionID: 4, IndividualWinCondition: "TeamFight", Scores: Scores{SplitPush: 0, Pick: 1, Siege: 1, Control: 4, TeamFight: 4}},
			{Champion: "Five", ChampionID: 5, IndividualWinCondition: "Pick", Scores: Scores{SplitPush: 1, Pick: 3, Siege: 0, Control: 2, TeamFight: 4}},
		},
	}

	profile := NewAnalyzer(catalog).TeamProfile([]uint16{1, 2, 3, 4, 5})

	if profile.Scores.Pick != 14 || profile.Scores.TeamFight != 18 {
		t.Fatalf("scores = %+v, want pick 14 and teamfight 18", profile.Scores)
	}
	if profile.PrimaryCondition != "TeamFight" {
		t.Fatalf("primary condition = %s, want TeamFight", profile.PrimaryCondition)
	}
	if profile.PrimaryRating != "A-" {
		t.Fatalf("primary rating = %s, want A-", profile.PrimaryRating)
	}
	if profile.Ratings["pick"] != "B" {
		t.Fatalf("pick rating = %s, want B", profile.Ratings["pick"])
	}
}

func TestTeamProfileTieBreakerUsesIndividualConditions(t *testing.T) {
	catalog := Catalog{
		Patch: "test",
		Profiles: []Profile{
			{ChampionID: 1, IndividualWinCondition: "Pick", Scores: Scores{Pick: 5, TeamFight: 5}},
			{ChampionID: 2, IndividualWinCondition: "Pick", Scores: Scores{Pick: 5, TeamFight: 5}},
			{ChampionID: 3, IndividualWinCondition: "TeamFight", Scores: Scores{Pick: 5, TeamFight: 5}},
		},
	}

	profile := NewAnalyzer(catalog).TeamProfile([]uint16{1, 2, 3})

	if profile.PrimaryCondition != "Pick" {
		t.Fatalf("primary condition = %s, want Pick", profile.PrimaryCondition)
	}
	if profile.PrimaryRating != "B+" {
		t.Fatalf("primary rating = %s, want B+", profile.PrimaryRating)
	}
}

func TestRatingClampsAboveLegacyScale(t *testing.T) {
	if got := Rating(31); got != "S+" {
		t.Fatalf("Rating(31) = %s, want S+", got)
	}
}
