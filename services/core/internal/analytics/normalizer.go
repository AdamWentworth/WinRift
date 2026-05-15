package analytics

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const (
	RankedSoloQueueID = 420
	SummonersRiftMap  = 11
)

var trinkets = map[int]bool{3340: true, 3363: true, 3364: true, 3330: true, 3348: true, 2052: true}

type NormalizedMatch struct {
	RawMatch                  RawMatch
	RawTimeline               RawTimeline
	Participants              []ParticipantRow
	Matchups                  []MatchupRow
	TimelineParticipantFrames []TimelineParticipantFrameRow
	TimelineItemEvents        []TimelineItemEventRow
	TimelineCombatEvents      []TimelineCombatEventRow
	TimelineObjectiveEvents   []TimelineObjectiveEventRow
}

type RawMatch struct {
	MatchID            string
	Platform           string
	QueueID            uint16
	Patch              string
	GameCreation       uint64
	GameStartTimestamp uint64
	GameEndTimestamp   uint64
	DurationSeconds    uint32
	RawJSON            string
}

type RawTimeline struct {
	MatchID  string
	Platform string
	Patch    string
	QueueID  uint16
	RawJSON  string
}

type ParticipantRow struct {
	MatchID             string
	Platform            string
	Patch               string
	QueueID             uint16
	ParticipantID       uint8
	PUUID               string
	TeamID              uint16
	ChampionID          uint16
	ChampionName        string
	Role                string
	Win                 uint8
	Kills               uint16
	Deaths              uint16
	Assists             uint16
	Item0               uint32
	Item1               uint32
	Item2               uint32
	Item3               uint32
	Item4               uint32
	Item5               uint32
	TrinketItem         uint32
	SummonerSpell1      uint16
	SummonerSpell2      uint16
	PrimaryRuneTree     uint16
	SecondaryRuneTree   uint16
	Keystone            uint16
	RuneSignature       string
	SpellSignature      string
	FinalItemsSignature string
	Core2Signature      string
	Core3Signature      string
	RankBucket          string
}

type MatchupRow struct {
	ParticipantRow
	OpponentParticipantID uint8
	OpponentChampionID    uint16
	OpponentRole          string
}

type TimelineParticipantFrameRow struct {
	MatchID                    string
	Platform                   string
	Patch                      string
	QueueID                    uint16
	TimestampMS                uint32
	ParticipantID              uint8
	Level                      uint8
	XP                         uint32
	CurrentGold                uint32
	TotalGold                  uint32
	MinionsKilled              uint32
	JungleMinionsKilled        uint32
	PositionX                  int32
	PositionY                  int32
	TotalDamageDoneToChampions uint32
	TotalDamageTaken           uint32
}

type TimelineItemEventRow struct {
	MatchID       string
	Platform      string
	Patch         string
	QueueID       uint16
	TimestampMS   uint32
	ParticipantID uint8
	EventType     string
	ItemID        uint32
	BeforeID      uint32
	AfterID       uint32
}

type TimelineCombatEventRow struct {
	MatchID                 string
	Platform                string
	Patch                   string
	QueueID                 uint16
	TimestampMS             uint32
	KillerID                uint8
	VictimID                uint8
	AssistingParticipantIDs string
	Bounty                  uint32
	ShutdownBounty          uint32
	PositionX               int32
	PositionY               int32
}

type TimelineObjectiveEventRow struct {
	MatchID        string
	Platform       string
	Patch          string
	QueueID        uint16
	TimestampMS    uint32
	EventType      string
	KillerID       uint8
	TeamID         uint16
	MonsterType    string
	MonsterSubType string
	BuildingType   string
	TowerType      string
	LaneType       string
	PositionX      int32
	PositionY      int32
}

type matchPayload struct {
	Metadata struct {
		MatchID string `json:"matchId"`
	} `json:"metadata"`
	Info matchInfo `json:"info"`
}

type matchInfo struct {
	GameCreation       uint64             `json:"gameCreation"`
	GameDuration       uint32             `json:"gameDuration"`
	GameEndTimestamp   uint64             `json:"gameEndTimestamp"`
	GameID             uint64             `json:"gameId"`
	GameMode           string             `json:"gameMode"`
	GameStartTimestamp uint64             `json:"gameStartTimestamp"`
	GameVersion        string             `json:"gameVersion"`
	MapID              int                `json:"mapId"`
	QueueID            int                `json:"queueId"`
	Participants       []matchParticipant `json:"participants"`
}

type matchParticipant struct {
	ParticipantID      int    `json:"participantId"`
	PUUID              string `json:"puuid"`
	TeamID             int    `json:"teamId"`
	ChampionID         int    `json:"championId"`
	ChampionName       string `json:"championName"`
	TeamPosition       string `json:"teamPosition"`
	IndividualPosition string `json:"individualPosition"`
	Lane               string `json:"lane"`
	Win                bool   `json:"win"`
	Kills              int    `json:"kills"`
	Deaths             int    `json:"deaths"`
	Assists            int    `json:"assists"`
	Item0              int    `json:"item0"`
	Item1              int    `json:"item1"`
	Item2              int    `json:"item2"`
	Item3              int    `json:"item3"`
	Item4              int    `json:"item4"`
	Item5              int    `json:"item5"`
	Item6              int    `json:"item6"`
	Summoner1ID        int    `json:"summoner1Id"`
	Summoner2ID        int    `json:"summoner2Id"`
	Spell1ID           int    `json:"spell1Id"`
	Spell2ID           int    `json:"spell2Id"`
	Perks              perks  `json:"perks"`
}

type perks struct {
	StatPerks    map[string]int `json:"statPerks"`
	Styles       []perkStyle    `json:"styles"`
	PerkIDs      []int          `json:"perkIds"`
	PerkStyle    int            `json:"perkStyle"`
	PerkSubStyle int            `json:"perkSubStyle"`
}

type perkStyle struct {
	Description string          `json:"description"`
	Style       int             `json:"style"`
	Selections  []perkSelection `json:"selections"`
}

type perkSelection struct {
	Perk int `json:"perk"`
}

type timelinePayload struct {
	Info struct {
		Frames []struct {
			Timestamp         int                                 `json:"timestamp"`
			ParticipantFrames map[string]timelineParticipantFrame `json:"participantFrames"`
			Events            []timelineEvent                     `json:"events"`
		} `json:"frames"`
	} `json:"info"`
}

type timelineParticipantFrame struct {
	ParticipantID       int              `json:"participantId"`
	Level               int              `json:"level"`
	XP                  int              `json:"xp"`
	CurrentGold         int              `json:"currentGold"`
	TotalGold           int              `json:"totalGold"`
	MinionsKilled       int              `json:"minionsKilled"`
	JungleMinionsKilled int              `json:"jungleMinionsKilled"`
	Position            timelinePosition `json:"position"`
	DamageStats         timelineDamage   `json:"damageStats"`
}

type timelineDamage struct {
	TotalDamageDoneToChampions int `json:"totalDamageDoneToChampions"`
	TotalDamageTaken           int `json:"totalDamageTaken"`
}

type timelineEvent struct {
	Type                    string           `json:"type"`
	Timestamp               int              `json:"timestamp"`
	ParticipantID           int              `json:"participantId"`
	ItemID                  int              `json:"itemId"`
	BeforeID                int              `json:"beforeId"`
	AfterID                 int              `json:"afterId"`
	KillerID                int              `json:"killerId"`
	VictimID                int              `json:"victimId"`
	AssistingParticipantIDs []int            `json:"assistingParticipantIds"`
	Bounty                  int              `json:"bounty"`
	ShutdownBounty          int              `json:"shutdownBounty"`
	TeamID                  int              `json:"teamId"`
	MonsterType             string           `json:"monsterType"`
	MonsterSubType          string           `json:"monsterSubType"`
	BuildingType            string           `json:"buildingType"`
	TowerType               string           `json:"towerType"`
	LaneType                string           `json:"laneType"`
	Position                timelinePosition `json:"position"`
}

type timelinePosition struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func NormalizeMatch(rawMatch, rawTimeline []byte, platform, rankBucket string) (NormalizedMatch, error) {
	var payload matchPayload
	if err := json.Unmarshal(rawMatch, &payload); err != nil {
		return NormalizedMatch{}, err
	}
	var timeline timelinePayload
	if len(rawTimeline) > 0 {
		_ = json.Unmarshal(rawTimeline, &timeline)
	}
	matchID := payload.Metadata.MatchID
	if matchID == "" {
		matchID = fmt.Sprintf("%s_%d", platform, payload.Info.GameID)
	}
	patch := PatchBucket(payload.Info.GameVersion)
	normalized := NormalizedMatch{
		RawMatch: RawMatch{
			MatchID:            matchID,
			Platform:           strings.ToUpper(platform),
			QueueID:            uint16(payload.Info.QueueID),
			Patch:              patch,
			GameCreation:       payload.Info.GameCreation,
			GameStartTimestamp: payload.Info.GameStartTimestamp,
			GameEndTimestamp:   payload.Info.GameEndTimestamp,
			DurationSeconds:    payload.Info.GameDuration,
			RawJSON:            string(rawMatch),
		},
		RawTimeline: RawTimeline{MatchID: matchID, Platform: strings.ToUpper(platform), Patch: patch, QueueID: uint16(payload.Info.QueueID), RawJSON: string(rawTimeline)},
	}

	if rankBucket == "" {
		rankBucket = "UNKNOWN"
	}
	for i, participant := range payload.Info.Participants {
		participantID := participant.ParticipantID
		if participantID == 0 {
			participantID = i + 1
		}
		finalSignature := FinalItemsSignature(participant)
		core2, core3 := CoreItemSignatures(timeline, participantID, finalSignature)
		row := ParticipantRow{
			MatchID:             matchID,
			Platform:            strings.ToUpper(platform),
			Patch:               patch,
			QueueID:             uint16(payload.Info.QueueID),
			ParticipantID:       uint8(participantID),
			PUUID:               participant.PUUID,
			TeamID:              uint16(participant.TeamID),
			ChampionID:          uint16(participant.ChampionID),
			ChampionName:        participant.ChampionName,
			Role:                NormalizeRole(participant),
			Win:                 boolToUint8(participant.Win),
			Kills:               uint16(participant.Kills),
			Deaths:              uint16(participant.Deaths),
			Assists:             uint16(participant.Assists),
			Item0:               uint32(participant.Item0),
			Item1:               uint32(participant.Item1),
			Item2:               uint32(participant.Item2),
			Item3:               uint32(participant.Item3),
			Item4:               uint32(participant.Item4),
			Item5:               uint32(participant.Item5),
			TrinketItem:         uint32(participant.Item6),
			SummonerSpell1:      uint16(firstNonZero(participant.Summoner1ID, participant.Spell1ID)),
			SummonerSpell2:      uint16(firstNonZero(participant.Summoner2ID, participant.Spell2ID)),
			PrimaryRuneTree:     uint16(PrimaryTree(participant.Perks)),
			SecondaryRuneTree:   uint16(SecondaryTree(participant.Perks)),
			Keystone:            uint16(Keystone(participant.Perks)),
			RuneSignature:       RuneSignature(participant.Perks),
			SpellSignature:      SpellSignature(firstNonZero(participant.Summoner1ID, participant.Spell1ID), firstNonZero(participant.Summoner2ID, participant.Spell2ID)),
			FinalItemsSignature: finalSignature,
			Core2Signature:      core2,
			Core3Signature:      core3,
			RankBucket:          strings.ToUpper(rankBucket),
		}
		normalized.Participants = append(normalized.Participants, row)
	}
	normalized.Matchups = BuildMatchupRows(normalized.Participants)
	normalized.TimelineParticipantFrames = BuildTimelineParticipantFrames(timeline, matchID, strings.ToUpper(platform), patch, uint16(payload.Info.QueueID))
	normalized.TimelineItemEvents = BuildTimelineItemEvents(timeline, matchID, strings.ToUpper(platform), patch, uint16(payload.Info.QueueID))
	normalized.TimelineCombatEvents = BuildTimelineCombatEvents(timeline, matchID, strings.ToUpper(platform), patch, uint16(payload.Info.QueueID))
	normalized.TimelineObjectiveEvents = BuildTimelineObjectiveEvents(timeline, matchID, strings.ToUpper(platform), patch, uint16(payload.Info.QueueID))
	return normalized, nil
}

func ShouldIngest(rawMatch []byte) bool {
	var payload matchPayload
	if err := json.Unmarshal(rawMatch, &payload); err != nil {
		return false
	}
	return payload.Info.QueueID == RankedSoloQueueID && payload.Info.MapID == SummonersRiftMap && payload.Info.GameMode == "CLASSIC"
}

func PatchBucket(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		if version == "" {
			return "unknown"
		}
		return version
	}
	return parts[0] + "." + parts[1]
}

func FinalItemsSignature(participant matchParticipant) string {
	items := []int{participant.Item0, participant.Item1, participant.Item2, participant.Item3, participant.Item4, participant.Item5}
	return joinNonZero(items)
}

func CoreItemSignatures(timeline timelinePayload, participantID int, finalSignature string) (string, string) {
	finalItems := map[int]bool{}
	for _, part := range strings.Split(finalSignature, "-") {
		if value, err := strconv.Atoi(part); err == nil {
			finalItems[value] = true
		}
	}
	purchased := []int{}
	seen := map[int]bool{}
	for _, frame := range timeline.Info.Frames {
		for _, event := range frame.Events {
			if event.Type != "ITEM_PURCHASED" || event.ParticipantID != participantID || event.ItemID == 0 || trinkets[event.ItemID] {
				continue
			}
			if len(finalItems) > 0 && !finalItems[event.ItemID] {
				continue
			}
			if !seen[event.ItemID] {
				purchased = append(purchased, event.ItemID)
				seen[event.ItemID] = true
			}
		}
	}
	if len(purchased) == 0 {
		for _, part := range strings.Split(finalSignature, "-") {
			if value, err := strconv.Atoi(part); err == nil {
				purchased = append(purchased, value)
			}
		}
	}
	return joinNonZero(limitInts(purchased, 2)), joinNonZero(limitInts(purchased, 3))
}

func RuneSignature(value perks) string {
	if len(value.PerkIDs) > 0 {
		ids := make([]string, 0, len(value.PerkIDs))
		for _, id := range value.PerkIDs {
			ids = append(ids, strconv.Itoa(id))
		}
		return strings.Join([]string{strconv.Itoa(value.PerkStyle), strconv.Itoa(value.PerkSubStyle), strings.Join(ids, "-")}, "|")
	}
	selections := []string{}
	for _, style := range value.Styles {
		for _, selection := range style.Selections {
			if selection.Perk != 0 {
				selections = append(selections, strconv.Itoa(selection.Perk))
			}
		}
	}
	stats := []string{strconv.Itoa(value.StatPerks["offense"]), strconv.Itoa(value.StatPerks["flex"]), strconv.Itoa(value.StatPerks["defense"])}
	return strings.Join([]string{strconv.Itoa(PrimaryTree(value)), strconv.Itoa(SecondaryTree(value)), strings.Join(selections, "-"), strings.Join(stats, "-")}, "|")
}

func SpellSignature(spell1, spell2 int) string {
	spells := []int{spell1, spell2}
	sort.Ints(spells)
	return joinNonZero(spells)
}

func NormalizeRole(participant matchParticipant) string {
	for _, value := range []string{participant.TeamPosition, participant.IndividualPosition, participant.Lane} {
		value = strings.ToUpper(strings.TrimSpace(value))
		if value != "" {
			return value
		}
	}
	return "UNKNOWN"
}

func PrimaryTree(value perks) int {
	if value.PerkStyle != 0 {
		return value.PerkStyle
	}
	if len(value.Styles) > 0 {
		return value.Styles[0].Style
	}
	return 0
}

func SecondaryTree(value perks) int {
	if value.PerkSubStyle != 0 {
		return value.PerkSubStyle
	}
	if len(value.Styles) > 1 {
		return value.Styles[1].Style
	}
	return 0
}

func Keystone(value perks) int {
	if len(value.PerkIDs) > 0 {
		return value.PerkIDs[0]
	}
	if len(value.Styles) > 0 && len(value.Styles[0].Selections) > 0 {
		return value.Styles[0].Selections[0].Perk
	}
	return 0
}

func BuildMatchupRows(participants []ParticipantRow) []MatchupRow {
	byTeamRole := map[string][]ParticipantRow{}
	byTeam := map[uint16][]ParticipantRow{}
	for _, participant := range participants {
		key := fmt.Sprintf("%d:%s", participant.TeamID, participant.Role)
		byTeamRole[key] = append(byTeamRole[key], participant)
		byTeam[participant.TeamID] = append(byTeam[participant.TeamID], participant)
	}
	rows := make([]MatchupRow, 0, len(participants))
	for _, participant := range participants {
		enemyTeam := uint16(100)
		if participant.TeamID == 100 {
			enemyTeam = 200
		}
		candidates := byTeamRole[fmt.Sprintf("%d:%s", enemyTeam, participant.Role)]
		if len(candidates) == 0 {
			candidates = byTeam[enemyTeam]
		}
		if len(candidates) == 0 {
			continue
		}
		opponent := candidates[0]
		rows = append(rows, MatchupRow{ParticipantRow: participant, OpponentParticipantID: opponent.ParticipantID, OpponentChampionID: opponent.ChampionID, OpponentRole: opponent.Role})
	}
	return rows
}

func BuildTimelineParticipantFrames(timeline timelinePayload, matchID, platform, patch string, queueID uint16) []TimelineParticipantFrameRow {
	rows := []TimelineParticipantFrameRow{}
	for _, frame := range timeline.Info.Frames {
		for _, participantFrame := range frame.ParticipantFrames {
			participantID := participantFrame.ParticipantID
			if participantID == 0 {
				continue
			}
			rows = append(rows, TimelineParticipantFrameRow{
				MatchID:                    matchID,
				Platform:                   platform,
				Patch:                      patch,
				QueueID:                    queueID,
				TimestampMS:                uint32NonNegative(frame.Timestamp),
				ParticipantID:              uint8NonNegative(participantID),
				Level:                      uint8NonNegative(participantFrame.Level),
				XP:                         uint32NonNegative(participantFrame.XP),
				CurrentGold:                uint32NonNegative(participantFrame.CurrentGold),
				TotalGold:                  uint32NonNegative(participantFrame.TotalGold),
				MinionsKilled:              uint32NonNegative(participantFrame.MinionsKilled),
				JungleMinionsKilled:        uint32NonNegative(participantFrame.JungleMinionsKilled),
				PositionX:                  int32(participantFrame.Position.X),
				PositionY:                  int32(participantFrame.Position.Y),
				TotalDamageDoneToChampions: uint32NonNegative(participantFrame.DamageStats.TotalDamageDoneToChampions),
				TotalDamageTaken:           uint32NonNegative(participantFrame.DamageStats.TotalDamageTaken),
			})
		}
	}
	return rows
}

func BuildTimelineItemEvents(timeline timelinePayload, matchID, platform, patch string, queueID uint16) []TimelineItemEventRow {
	rows := []TimelineItemEventRow{}
	for _, frame := range timeline.Info.Frames {
		for _, event := range frame.Events {
			if !isItemEvent(event.Type) {
				continue
			}
			timestamp := event.Timestamp
			if timestamp == 0 {
				timestamp = frame.Timestamp
			}
			rows = append(rows, TimelineItemEventRow{
				MatchID:       matchID,
				Platform:      platform,
				Patch:         patch,
				QueueID:       queueID,
				TimestampMS:   uint32NonNegative(timestamp),
				ParticipantID: uint8NonNegative(event.ParticipantID),
				EventType:     event.Type,
				ItemID:        uint32NonNegative(event.ItemID),
				BeforeID:      uint32NonNegative(event.BeforeID),
				AfterID:       uint32NonNegative(event.AfterID),
			})
		}
	}
	return rows
}

func BuildTimelineCombatEvents(timeline timelinePayload, matchID, platform, patch string, queueID uint16) []TimelineCombatEventRow {
	rows := []TimelineCombatEventRow{}
	for _, frame := range timeline.Info.Frames {
		for _, event := range frame.Events {
			if event.Type != "CHAMPION_KILL" {
				continue
			}
			timestamp := event.Timestamp
			if timestamp == 0 {
				timestamp = frame.Timestamp
			}
			rows = append(rows, TimelineCombatEventRow{
				MatchID:                 matchID,
				Platform:                platform,
				Patch:                   patch,
				QueueID:                 queueID,
				TimestampMS:             uint32NonNegative(timestamp),
				KillerID:                uint8NonNegative(event.KillerID),
				VictimID:                uint8NonNegative(event.VictimID),
				AssistingParticipantIDs: joinNonZero(event.AssistingParticipantIDs),
				Bounty:                  uint32NonNegative(event.Bounty),
				ShutdownBounty:          uint32NonNegative(event.ShutdownBounty),
				PositionX:               int32(event.Position.X),
				PositionY:               int32(event.Position.Y),
			})
		}
	}
	return rows
}

func BuildTimelineObjectiveEvents(timeline timelinePayload, matchID, platform, patch string, queueID uint16) []TimelineObjectiveEventRow {
	rows := []TimelineObjectiveEventRow{}
	for _, frame := range timeline.Info.Frames {
		for _, event := range frame.Events {
			if !isObjectiveEvent(event.Type) {
				continue
			}
			timestamp := event.Timestamp
			if timestamp == 0 {
				timestamp = frame.Timestamp
			}
			rows = append(rows, TimelineObjectiveEventRow{
				MatchID:        matchID,
				Platform:       platform,
				Patch:          patch,
				QueueID:        queueID,
				TimestampMS:    uint32NonNegative(timestamp),
				EventType:      event.Type,
				KillerID:       uint8NonNegative(event.KillerID),
				TeamID:         uint16NonNegative(event.TeamID),
				MonsterType:    event.MonsterType,
				MonsterSubType: event.MonsterSubType,
				BuildingType:   event.BuildingType,
				TowerType:      event.TowerType,
				LaneType:       event.LaneType,
				PositionX:      int32(event.Position.X),
				PositionY:      int32(event.Position.Y),
			})
		}
	}
	return rows
}

func isItemEvent(eventType string) bool {
	switch eventType {
	case "ITEM_PURCHASED", "ITEM_SOLD", "ITEM_DESTROYED", "ITEM_UNDO":
		return true
	default:
		return false
	}
}

func isObjectiveEvent(eventType string) bool {
	switch eventType {
	case "ELITE_MONSTER_KILL", "BUILDING_KILL", "TURRET_PLATE_DESTROYED":
		return true
	default:
		return false
	}
}

func WilsonLowerBound(wins, games int, z float64) float64 {
	if games <= 0 {
		return 0
	}
	phat := float64(wins) / float64(games)
	denominator := 1 + z*z/float64(games)
	centre := phat + z*z/(2*float64(games))
	margin := z * math.Sqrt((phat*(1-phat)+z*z/(4*float64(games)))/float64(games))
	return (centre - margin) / denominator
}

func joinNonZero(items []int) string {
	parts := []string{}
	for _, item := range items {
		if item != 0 {
			parts = append(parts, strconv.Itoa(item))
		}
	}
	return strings.Join(parts, "-")
}

func limitInts(items []int, limit int) []int {
	if len(items) < limit {
		return items
	}
	return items[:limit]
}

func boolToUint8(value bool) uint8 {
	if value {
		return 1
	}
	return 0
}

func firstNonZero(values ...int) int {
	for _, value := range values {
		if value != 0 {
			return value
		}
	}
	return 0
}

func uint8NonNegative(value int) uint8 {
	if value < 0 {
		return 0
	}
	return uint8(value)
}

func uint16NonNegative(value int) uint16 {
	if value < 0 {
		return 0
	}
	return uint16(value)
}

func uint32NonNegative(value int) uint32 {
	if value < 0 {
		return 0
	}
	return uint32(value)
}
