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

type Axis struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type AxisScore struct {
	Key              string `json:"key"`
	Label            string `json:"label"`
	Score            int    `json:"score"`
	Rating           string `json:"rating"`
	DeltaFromPrimary int    `json:"deltaFromPrimary"`
	PlanRole         string `json:"planRole"`
	PlanLabel        string `json:"planLabel"`
}

type TeamProfile struct {
	ChampionIDs        []uint16          `json:"championIds"`
	Scores             Scores            `json:"scores"`
	Ratings            map[string]string `json:"ratings"`
	Axes               []AxisScore       `json:"axes"`
	PrimaryCondition   string            `json:"primaryCondition"`
	PrimaryScore       int               `json:"primaryScore"`
	PrimaryRating      string            `json:"primaryRating"`
	PrimaryMargin      int               `json:"primaryMargin"`
	Sharpness          string            `json:"sharpness"`
	SharpnessLabel     string            `json:"sharpnessLabel"`
	MissingChampionIDs []uint16          `json:"missingChampionIds"`
}

type Analyzer struct {
	catalog      Catalog
	profilesByID map[uint16]Profile
}

var Axes = []Axis{
	{Key: "splitpush", Label: "SplitPush"},
	{Key: "pick", Label: "Pick"},
	{Key: "siege", Label: "Siege"},
	{Key: "control", Label: "Control"},
	{Key: "teamfight", Label: "TeamFight"},
}

func LoadCatalog() (Catalog, error) {
	var catalog Catalog
	if err := json.Unmarshal(embeddedProfiles, &catalog); err != nil {
		return Catalog{}, fmt.Errorf("decode win condition profiles: %w", err)
	}
	return catalog, nil
}

func NewAnalyzer(catalog Catalog) Analyzer {
	profilesByID := make(map[uint16]Profile, len(catalog.Profiles))
	for _, profile := range catalog.Profiles {
		profilesByID[profile.ChampionID] = profile
	}
	return Analyzer{catalog: catalog, profilesByID: profilesByID}
}

func (a Analyzer) CatalogPatch() string {
	return a.catalog.Patch
}

func (a Analyzer) TeamProfile(championIDs []uint16) TeamProfile {
	scores := Scores{}
	profiles := make([]Profile, 0, len(championIDs))
	missing := make([]uint16, 0)
	for _, championID := range championIDs {
		profile, ok := a.profilesByID[championID]
		if !ok {
			missing = append(missing, championID)
			continue
		}
		profiles = append(profiles, profile)
		scores.SplitPush += profile.Scores.SplitPush
		scores.Pick += profile.Scores.Pick
		scores.Siege += profile.Scores.Siege
		scores.Control += profile.Scores.Control
		scores.TeamFight += profile.Scores.TeamFight
	}
	primaryCondition, primaryScore := primaryCondition(scores, profiles)
	ratings := scoreRatings(scores)
	primaryRating := "D-"
	if primaryCondition != "Unknown" {
		primaryRating = ratings[keyForLabel(primaryCondition)]
	}
	primaryMargin := primaryMargin(scores)
	return TeamProfile{
		ChampionIDs:        append([]uint16(nil), championIDs...),
		Scores:             scores,
		Ratings:            ratings,
		Axes:               axisScores(scores, primaryCondition, primaryScore),
		PrimaryCondition:   primaryCondition,
		PrimaryScore:       primaryScore,
		PrimaryRating:      primaryRating,
		PrimaryMargin:      primaryMargin,
		Sharpness:          sharpness(primaryMargin),
		SharpnessLabel:     sharpnessLabel(primaryMargin),
		MissingChampionIDs: missing,
	}
}

func Rating(score int) string {
	switch {
	case score <= 0:
		return "D-"
	case score <= 2:
		return "D"
	case score <= 4:
		return "D+"
	case score <= 6:
		return "C-"
	case score <= 8:
		return "C"
	case score <= 10:
		return "C+"
	case score <= 12:
		return "B-"
	case score <= 14:
		return "B"
	case score <= 16:
		return "B+"
	case score <= 18:
		return "A-"
	case score <= 20:
		return "A"
	case score <= 22:
		return "A+"
	case score == 23:
		return "S-"
	case score == 24:
		return "S"
	default:
		return "S+"
	}
}

func (scores Scores) Value(label string) int {
	switch label {
	case "SplitPush":
		return scores.SplitPush
	case "Pick":
		return scores.Pick
	case "Siege":
		return scores.Siege
	case "Control":
		return scores.Control
	case "TeamFight":
		return scores.TeamFight
	default:
		return 0
	}
}

func (scores Scores) Total() int {
	return scores.SplitPush + scores.Pick + scores.Siege + scores.Control + scores.TeamFight
}

func scoreRatings(scores Scores) map[string]string {
	return map[string]string{
		"splitpush": Rating(scores.SplitPush),
		"pick":      Rating(scores.Pick),
		"siege":     Rating(scores.Siege),
		"control":   Rating(scores.Control),
		"teamfight": Rating(scores.TeamFight),
	}
}

func axisScores(scores Scores, primaryCondition string, primaryScore int) []AxisScore {
	out := make([]AxisScore, 0, len(Axes))
	for _, axis := range Axes {
		score := scores.Value(axis.Label)
		delta := primaryScore - score
		if delta < 0 {
			delta = 0
		}
		role, label := planRole(axis.Label, score, primaryCondition, primaryScore)
		out = append(out, AxisScore{
			Key:              axis.Key,
			Label:            axis.Label,
			Score:            score,
			Rating:           Rating(score),
			DeltaFromPrimary: delta,
			PlanRole:         role,
			PlanLabel:        label,
		})
	}
	return out
}

func planRole(label string, score int, primaryCondition string, primaryScore int) (string, string) {
	if primaryCondition == "Unknown" {
		return "unknown", "Unknown"
	}
	if label == primaryCondition {
		return "primary", "Primary"
	}
	delta := primaryScore - score
	switch {
	case delta <= 0:
		return "co-primary", "Co-primary"
	case delta <= 2:
		return "strong-secondary", "Strong secondary"
	case delta <= 5 || score >= 12:
		return "secondary", "Secondary"
	default:
		return "weak-angle", "Weak angle"
	}
}

func primaryMargin(scores Scores) int {
	values := []int{scores.SplitPush, scores.Pick, scores.Siege, scores.Control, scores.TeamFight}
	best := -1
	second := -1
	for _, value := range values {
		if value > best {
			second = best
			best = value
			continue
		}
		if value > second {
			second = value
		}
	}
	if best < 0 {
		return 0
	}
	if second < 0 {
		return best
	}
	return best - second
}

func sharpness(margin int) string {
	switch {
	case margin <= 0:
		return "tied"
	case margin == 1:
		return "contested"
	case margin <= 3:
		return "flexible"
	case margin <= 6:
		return "clear"
	default:
		return "sharp"
	}
}

func sharpnessLabel(margin int) string {
	switch sharpness(margin) {
	case "tied":
		return "Tied identity"
	case "contested":
		return "Contested identity"
	case "flexible":
		return "Flexible identity"
	case "clear":
		return "Clear identity"
	default:
		return "Sharp identity"
	}
}

func primaryCondition(scores Scores, profiles []Profile) (string, int) {
	maxScore := -1
	tied := make([]Axis, 0, len(Axes))
	for _, axis := range Axes {
		score := scores.Value(axis.Label)
		switch {
		case score > maxScore:
			maxScore = score
			tied = []Axis{axis}
		case score == maxScore:
			tied = append(tied, axis)
		}
	}
	if maxScore <= 0 && len(profiles) == 0 {
		return "Unknown", 0
	}
	if len(tied) == 1 {
		return tied[0].Label, maxScore
	}
	tiedLabels := make(map[string]bool, len(tied))
	for _, axis := range tied {
		tiedLabels[axis.Label] = true
	}
	individualCounts := make(map[string]int, len(tied))
	for _, profile := range profiles {
		if tiedLabels[profile.IndividualWinCondition] {
			individualCounts[profile.IndividualWinCondition]++
		}
	}
	bestLabel := tied[0].Label
	bestCount := individualCounts[bestLabel]
	for _, axis := range tied[1:] {
		count := individualCounts[axis.Label]
		if count > bestCount {
			bestLabel = axis.Label
			bestCount = count
		}
	}
	return bestLabel, maxScore
}

func keyForLabel(label string) string {
	for _, axis := range Axes {
		if axis.Label == label {
			return axis.Key
		}
	}
	return ""
}
