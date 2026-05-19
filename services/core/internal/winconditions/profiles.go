package winconditions

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed champion_profiles.json
var embeddedProfiles []byte

type Catalog struct {
	SchemaVersion int       `json:"schemaVersion"`
	Patch         string    `json:"patch"`
	Axes          []string  `json:"axes"`
	Profiles      []Profile `json:"profiles"`
}

type Profile struct {
	Champion               string `json:"champion"`
	ChampionID             uint16 `json:"championId"`
	IndividualWinCondition string `json:"individualWinCondition"`
	Source                 string `json:"source"`
	Scores                 Scores `json:"scores"`
}

type Scores struct {
	SplitPush int `json:"splitpush"`
	Pick      int `json:"pick"`
	Siege     int `json:"siege"`
	Control   int `json:"control"`
	TeamFight int `json:"teamfight"`
}

func LoadCatalog() (Catalog, error) {
	var catalog Catalog
	if err := json.Unmarshal(embeddedProfiles, &catalog); err != nil {
		return Catalog{}, fmt.Errorf("decode win condition profiles: %w", err)
	}
	return catalog, nil
}

func (scores Scores) Total() int {
	return scores.SplitPush + scores.Pick + scores.Siege + scores.Control + scores.TeamFight
}
