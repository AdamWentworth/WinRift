package winconditions

import "fmt"

type NarrativeBucket struct {
	Bucket        string  `json:"bucket"`
	Wins          int     `json:"wins"`
	Games         int     `json:"games"`
	WinRate       float64 `json:"winRate"`
	MeetsMinGames bool    `json:"meetsMinGames"`
}

type NarrativeMetric struct {
	Condition         string            `json:"condition"`
	Rating            string            `json:"rating"`
	PlanRole          string            `json:"planRole"`
	PlanLabel         string            `json:"planLabel"`
	OpponentCondition string            `json:"opponentCondition"`
	OpponentRating    string            `json:"opponentRating"`
	OpponentPlanRole  string            `json:"opponentPlanRole"`
	OpponentPlanLabel string            `json:"opponentPlanLabel"`
	Primary           bool              `json:"primary"`
	OpponentPrimary   bool              `json:"opponentPrimary"`
	Wins              int               `json:"wins"`
	Games             int               `json:"games"`
	WinRate           float64           `json:"winRate"`
	Confidence        float64           `json:"confidence"`
	MeetsMinGames     bool              `json:"meetsMinGames"`
	Buckets           []NarrativeBucket `json:"buckets"`
}

type NarrativeScript struct {
	ID          string   `json:"id"`
	Headline    string   `json:"headline"`
	Overview    string   `json:"overview"`
	Matchup     string   `json:"matchup"`
	RatingRead  string   `json:"ratingRead"`
	ModeRead    string   `json:"modeRead"`
	TimingRead  string   `json:"timingRead"`
	CautionRead string   `json:"cautionRead"`
	SampleRead  string   `json:"sampleRead"`
	PlayerRead  string   `json:"playerRead"`
	Facts       []string `json:"facts"`
}

var RatingLabels = []string{"D-", "D", "D+", "C-", "C", "C+", "B-", "B", "B+", "A-", "A", "A+", "S-", "S", "S+"}

var ratingIndex = map[string]int{
	"D-": 0, "D": 1, "D+": 2,
	"C-": 3, "C": 4, "C+": 5,
	"B-": 6, "B": 7, "B+": 8,
	"A-": 9, "A": 10, "A+": 11,
	"S-": 12, "S": 13, "S+": 14,
}

var conditionReads = map[string]string{
	"SplitPush": "side-lane pressure, tower trading, and forcing the enemy to answer away from the main group",
	"Pick":      "catching isolated targets, starting fights on your terms, and punishing poor vision",
	"Siege":     "grouping around waves, hitting structures, and making the enemy defend under pressure",
	"Control":   "owning space around objectives, choking vision, and making fights hard to enter",
	"TeamFight": "grouped fights, layered engage, and coordinated damage around major objectives",
}

var matchupReads = map[string]map[string]string{
	"SplitPush": {
		"SplitPush": "Both teams want side-lane pressure, so the matchup often comes down to who manages waves and teleport windows better.",
		"Pick":      "Splitpush can stretch pick comps thin, but careless side-lane movement gives the pick team the catches it wants.",
		"Siege":     "Splitpush tries to pull defenders away before the siege group can settle in front of towers.",
		"Control":   "Splitpush tests whether the control team can hold objective space while still answering side lanes.",
		"TeamFight": "Splitpush tries to avoid the clean grouped fight and force the teamfight comp to choose between waves and objectives.",
	},
	"Pick": {
		"SplitPush": "Pick looks to catch the splitpush setup while champions rotate between side lanes and the main group.",
		"Pick":      "Both teams are looking for mistakes, so vision control and patience matter more than raw engage.",
		"Siege":     "Pick can break siege before it starts by catching the champions moving to set up the wave.",
		"Control":   "Pick has to find angles before control tools lock the area down and make entry predictable.",
		"TeamFight": "Pick wants to remove a target before the teamfight comp can group and play front-to-back.",
	},
	"Siege": {
		"SplitPush": "Siege wants grouped pressure fast enough that the splitpush side cannot trade towers for free.",
		"Pick":      "Siege has to respect fog and flank paths because pick comps punish the setup phase.",
		"Siege":     "Both teams want wave and tower pressure, so range, waveclear, and poke discipline decide a lot.",
		"Control":   "Siege challenges control by forcing defenders to choose between clearing waves and holding objective space.",
		"TeamFight": "Siege wants to damage towers and champions before the teamfight comp gets a clean engage.",
	},
	"Control": {
		"SplitPush": "Control tries to make the map small, secure objective entrances, and punish the splitpush team when it rotates.",
		"Pick":      "Control denies pick angles by controlling entrances and making the enemy walk through known space.",
		"Siege":     "Control looks to slow the siege setup, clear vision, and choose when the tower defense turns into a fight.",
		"Control":   "Both teams want space first, so the matchup is often about vision timing and who starts setup earlier.",
		"TeamFight": "Control can make teamfight comps enter through bad angles, but it must not give up a clean engage.",
	},
	"TeamFight": {
		"SplitPush": "Teamfight wants to force grouped objectives before splitpush pressure creates an uneven map.",
		"Pick":      "Teamfight can overpower pick if it groups cleanly, but one caught carry can ruin the setup.",
		"Siege":     "Teamfight looks for the engage window before siege damage makes the fight too costly.",
		"Control":   "Teamfight needs a reliable entry point against control; walking in late usually favors the control side.",
		"TeamFight": "Both teams want grouped fights, so engage timing, target focus, and cooldown tracking carry the matchup.",
	},
}

var ratingReads = map[string]string{
	"D-": "almost nonexistent",
	"D":  "very light",
	"D+": "light",
	"C-": "below average",
	"C":  "average",
	"C+": "solid",
	"B-": "respectable",
	"B":  "good",
	"B+": "strong",
	"A-": "very strong",
	"A":  "excellent",
	"A+": "elite",
	"S-": "exceptional",
	"S":  "dominant",
	"S+": "overwhelming",
}

func AllNarrativeScripts() []NarrativeScript {
	out := make([]NarrativeScript, 0, len(Axes)*len(Axes)*len(RatingLabels)*len(RatingLabels)*4)
	for _, condition := range Axes {
		for _, opponentCondition := range Axes {
			for _, rating := range RatingLabels {
				for _, opponentRating := range RatingLabels {
					for _, primary := range []bool{true, false} {
						for _, opponentPrimary := range []bool{true, false} {
							out = append(out, BuildNarrative(NarrativeMetric{
								Condition:         condition.Label,
								Rating:            rating,
								OpponentCondition: opponentCondition.Label,
								OpponentRating:    opponentRating,
								Primary:           primary,
								OpponentPrimary:   opponentPrimary,
								Wins:              6,
								Games:             10,
								WinRate:           60,
								Confidence:        31.27,
								MeetsMinGames:     true,
								Buckets: []NarrativeBucket{
									{Bucket: "0-20", Wins: 1, Games: 2, WinRate: 50, MeetsMinGames: false},
									{Bucket: "20-25", Wins: 2, Games: 3, WinRate: 66.67, MeetsMinGames: false},
								},
							}))
						}
					}
				}
			}
		}
	}
	return out
}

func BuildNarrative(metric NarrativeMetric) NarrativeScript {
	headline := fmt.Sprintf("%s %s into %s %s", metric.Condition, metric.Rating, metric.OpponentCondition, metric.OpponentRating)
	overview := fmt.Sprintf("Your selected angle is %s: %s.", metric.Condition, conditionRead(metric.Condition))
	matchup := matchupRead(metric.Condition, metric.OpponentCondition)
	ratingRead := buildRatingRead(metric.Rating, metric.OpponentRating)
	modeRead := buildModeRead(metric.Primary, metric.OpponentPrimary)
	timingRead := buildTimingRead(metric.Buckets)
	cautionRead := buildCautionRead(metric)
	sampleRead := buildSampleRead(metric)
	playerRead := buildPlayerRead(metric)
	return NarrativeScript{
		ID:          narrativeID(metric),
		Headline:    headline,
		Overview:    overview,
		Matchup:     matchup,
		RatingRead:  ratingRead,
		ModeRead:    modeRead,
		TimingRead:  timingRead,
		CautionRead: cautionRead,
		SampleRead:  sampleRead,
		PlayerRead:  playerRead,
		Facts: []string{
			fmt.Sprintf("team_condition=%s", metric.Condition),
			fmt.Sprintf("team_rating=%s", metric.Rating),
			fmt.Sprintf("team_plan_role=%s", metric.PlanRole),
			fmt.Sprintf("opponent_condition=%s", metric.OpponentCondition),
			fmt.Sprintf("opponent_rating=%s", metric.OpponentRating),
			fmt.Sprintf("opponent_plan_role=%s", metric.OpponentPlanRole),
			fmt.Sprintf("team_primary=%t", metric.Primary),
			fmt.Sprintf("opponent_primary=%t", metric.OpponentPrimary),
			fmt.Sprintf("wins=%d", metric.Wins),
			fmt.Sprintf("games=%d", metric.Games),
			fmt.Sprintf("win_rate_percent=%.2f", metric.WinRate),
			fmt.Sprintf("wilson_confidence_percent=%.2f", metric.Confidence),
		},
	}
}

func conditionRead(condition string) string {
	if read, ok := conditionReads[condition]; ok {
		return read
	}
	return "a team identity that needs more catalog detail"
}

func matchupRead(condition, opponentCondition string) string {
	if reads, ok := matchupReads[condition]; ok {
		if read, ok := reads[opponentCondition]; ok {
			return read
		}
	}
	return "This matchup is best read through the selected team identities, sample size, and game-length pattern."
}

func buildRatingRead(rating, opponentRating string) string {
	ratingText := ratingReads[rating]
	if ratingText == "" {
		ratingText = "unrated"
	}
	opponentText := ratingReads[opponentRating]
	if opponentText == "" {
		opponentText = "unrated"
	}
	diff := ratingIndex[rating] - ratingIndex[opponentRating]
	switch {
	case diff >= 3:
		return fmt.Sprintf("The rating edge is clearly in your favor: your %s is %s while the enemy's %s is %s.", rating, ratingText, opponentRating, opponentText)
	case diff >= 1:
		return fmt.Sprintf("You have a modest rating edge here: %s is %s into their %s %s.", rating, ratingText, opponentRating, opponentText)
	case diff == 0:
		return fmt.Sprintf("The ratings are even on paper: both sides are graded %s, which usually shifts importance toward execution and timing.", rating)
	case diff <= -3:
		return fmt.Sprintf("The enemy has the clear rating edge: your %s is %s into their %s %s.", rating, ratingText, opponentRating, opponentText)
	default:
		return fmt.Sprintf("The enemy has a modest rating edge: your %s is %s into their %s %s.", rating, ratingText, opponentRating, opponentText)
	}
}

func buildModeRead(primary, opponentPrimary bool) string {
	switch {
	case primary && opponentPrimary:
		return "This is main plan into main plan, so the read compares what each composition is most naturally built to do."
	case primary && !opponentPrimary:
		return "This is your main plan into one of the enemy's secondary angles, so the enemy may not be fully built around that response."
	case !primary && opponentPrimary:
		return "This is one of your secondary angles into the enemy's main plan, so treat it as an available path rather than the default identity of the comp."
	default:
		return "This is secondary angle into secondary angle, which is useful context but usually less reliable than the primary-plan read."
	}
}

func buildTimingRead(buckets []NarrativeBucket) string {
	best, worst, ok := bestAndWorstBuckets(buckets)
	if !ok {
		return "There is not enough game-length data yet to call out a timing pattern."
	}
	if best.Bucket == worst.Bucket {
		return fmt.Sprintf("The available timing sample is concentrated around games ending %s, where this read is at %.0f%%.", best.Bucket, best.WinRate)
	}
	return fmt.Sprintf("The strongest timing bucket is games ending %s at %.0f%%, while the weakest bucket is %s at %.0f%%.", best.Bucket, best.WinRate, worst.Bucket, worst.WinRate)
}

func buildSampleRead(metric NarrativeMetric) string {
	if metric.Games == 0 {
		return "No historical matches matched this exact read yet, so treat it as theory until more data lands."
	}
	if !metric.MeetsMinGames {
		return fmt.Sprintf("This is a thin sample: %d games at %.0f%%. It can hint at a pattern, but it should not drive decisions by itself.", metric.Games, metric.WinRate)
	}
	if metric.Confidence < 35 {
		return fmt.Sprintf("The raw sample is %d games at %.0f%%, but confidence is still low; read it as weak evidence.", metric.Games, metric.WinRate)
	}
	return fmt.Sprintf("The sample is %d games at %.0f%% with a %.0f%% lower-bound confidence score.", metric.Games, metric.WinRate, metric.Confidence)
}

func buildCautionRead(metric NarrativeMetric) string {
	if !looksLikeCorrelationRisk(metric) {
		return ""
	}
	return fmt.Sprintf(
		"Caution: %s %s is labeled as %s for this composition, so the elevated %.0f%% result is more likely correlation than proof that these teams actually won by forcing %s. Treat it as a historical pattern to investigate, not a causal instruction.",
		metric.Condition,
		metric.Rating,
		readablePlanRole(metric),
		metric.WinRate,
		metric.Condition,
	)
}

func looksLikeCorrelationRisk(metric NarrativeMetric) bool {
	if metric.Games < 20 || metric.WinRate < 55 {
		return false
	}
	if weakPlanRole(metric.PlanRole) {
		return true
	}
	return ratingIndex[metric.Rating] <= ratingIndex["C-"] && !metric.Primary
}

func weakPlanRole(role string) bool {
	switch role {
	case "weak-angle":
		return true
	default:
		return false
	}
}

func readablePlanRole(metric NarrativeMetric) string {
	if metric.PlanLabel != "" {
		return metric.PlanLabel
	}
	if metric.Primary {
		return "Primary"
	}
	return "Alternative"
}

func buildPlayerRead(metric NarrativeMetric) string {
	if metric.Games == 0 {
		return "Play the composition's natural identity first, and revisit this matchup once the data pool grows."
	}
	switch {
	case metric.WinRate >= 57:
		return "In plain terms: this selected angle has been favorable, so lean into the conditions that make it happen."
	case metric.WinRate >= 52:
		return "In plain terms: this selected angle has been slightly favorable, but the game still needs clean execution."
	case metric.WinRate > 48:
		return "In plain terms: this looks close to even, so execution, lane state, and objective setup probably matter more than the label."
	case metric.WinRate > 43:
		return "In plain terms: this selected angle has been slightly unfavorable, so look for another path if the game state allows it."
	default:
		return "In plain terms: this selected angle has been difficult historically, so avoid forcing the game to revolve around it."
	}
}

func bestAndWorstBuckets(buckets []NarrativeBucket) (NarrativeBucket, NarrativeBucket, bool) {
	best := NarrativeBucket{}
	worst := NarrativeBucket{}
	found := false
	for _, bucket := range buckets {
		if bucket.Games <= 0 {
			continue
		}
		if !found || bucket.WinRate > best.WinRate || (bucket.WinRate == best.WinRate && bucket.Games > best.Games) {
			best = bucket
		}
		if !found || bucket.WinRate < worst.WinRate || (bucket.WinRate == worst.WinRate && bucket.Games > worst.Games) {
			worst = bucket
		}
		found = true
	}
	return best, worst, found
}

func narrativeID(metric NarrativeMetric) string {
	teamMode := "alternative"
	if metric.Primary {
		teamMode = "primary"
	}
	opponentMode := "alternative"
	if metric.OpponentPrimary {
		opponentMode = "primary"
	}
	return fmt.Sprintf("%s_%s_%s_vs_%s_%s_%s", metric.Condition, metric.Rating, teamMode, metric.OpponentCondition, metric.OpponentRating, opponentMode)
}
