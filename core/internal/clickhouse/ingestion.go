package clickhouse

import (
	"context"
	"database/sql"
	"strings"

	"winrift/core/internal/analytics"
)

const maxBatchInsertRows = 500

func (r *Repository) InsertNormalized(ctx context.Context, normalized analytics.NormalizedMatch) error {
	if err := r.insertRawMatch(ctx, normalized.RawMatch); err != nil {
		return err
	}
	if err := r.insertRawTimeline(ctx, normalized.RawTimeline); err != nil {
		return err
	}
	if err := r.insertParticipants(ctx, normalized.Participants); err != nil {
		return err
	}
	if err := r.insertParticipantPerformances(ctx, normalized.Participants); err != nil {
		return err
	}
	if err := r.insertMatchups(ctx, normalized.Matchups); err != nil {
		return err
	}
	if err := r.insertTimelineParticipantFrames(ctx, normalized.TimelineParticipantFrames); err != nil {
		return err
	}
	if err := r.insertTimelineItemEvents(ctx, normalized.TimelineItemEvents); err != nil {
		return err
	}
	if err := r.insertTimelineSkillEvents(ctx, normalized.TimelineSkillEvents); err != nil {
		return err
	}
	if err := r.insertTimelineCombatEvents(ctx, normalized.TimelineCombatEvents); err != nil {
		return err
	}
	if err := r.insertTimelineObjectiveEvents(ctx, normalized.TimelineObjectiveEvents); err != nil {
		return err
	}
	if err := r.insertChampionBans(ctx, normalized.ChampionBans); err != nil {
		return err
	}
	return nil
}

func (r *Repository) insertRawMatch(ctx context.Context, row analytics.RawMatch) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO raw_matches (match_id, platform, queue_id, patch, game_creation, game_start_timestamp, game_end_timestamp, duration_seconds, raw_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, row.MatchID, row.Platform, row.QueueID, row.Patch, row.GameCreation, row.GameStartTimestamp, row.GameEndTimestamp, row.DurationSeconds, row.RawJSON)
	return err
}

func (r *Repository) insertRawTimeline(ctx context.Context, row analytics.RawTimeline) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO raw_timelines (match_id, platform, patch, queue_id, raw_json) VALUES (?, ?, ?, ?, ?)`, row.MatchID, row.Platform, row.Patch, row.QueueID, row.RawJSON)
	return err
}

func (r *Repository) insertParticipants(ctx context.Context, rows []analytics.ParticipantRow) error {
	return insertRowsInBatches(ctx, r.db, `INSERT INTO participants (`+participantColumns+`) VALUES `, participantPlaceholders, len(rows), func(i int) []any {
		return participantArgs(rows[i])
	})
}

func (r *Repository) insertParticipantPerformances(ctx context.Context, rows []analytics.ParticipantRow) error {
	return insertRowsInBatches(ctx, r.db, `INSERT INTO participant_performance (`+participantPerformanceColumns+`) VALUES `, participantPerformancePlaceholders, len(rows), func(i int) []any {
		return participantPerformanceArgs(rows[i])
	})
}

func (r *Repository) insertMatchups(ctx context.Context, rows []analytics.MatchupRow) error {
	return insertRowsInBatches(ctx, r.db, `INSERT INTO participant_matchups (`+participantColumns+`, opponent_participant_id, opponent_champion_id, opponent_role) VALUES `, participantPlaceholders+`, ?, ?, ?`, len(rows), func(i int) []any {
		args := participantArgs(rows[i].ParticipantRow)
		return append(args, rows[i].OpponentParticipantID, rows[i].OpponentChampionID, rows[i].OpponentRole)
	})
}

func (r *Repository) insertTimelineParticipantFrames(ctx context.Context, rows []analytics.TimelineParticipantFrameRow) error {
	return insertRowsInBatches(ctx, r.db, `INSERT INTO timeline_participant_frames (match_id, platform, patch, queue_id, timestamp_ms, participant_id, level, xp, current_gold, total_gold, minions_killed, jungle_minions_killed, position_x, position_y, total_damage_done_to_champions, total_damage_taken) VALUES `, `?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?`, len(rows), func(i int) []any {
		row := rows[i]
		return []any{row.MatchID, row.Platform, row.Patch, row.QueueID, row.TimestampMS, row.ParticipantID, row.Level, row.XP, row.CurrentGold, row.TotalGold, row.MinionsKilled, row.JungleMinionsKilled, row.PositionX, row.PositionY, row.TotalDamageDoneToChampions, row.TotalDamageTaken}
	})
}

func (r *Repository) insertTimelineItemEvents(ctx context.Context, rows []analytics.TimelineItemEventRow) error {
	return insertRowsInBatches(ctx, r.db, `INSERT INTO timeline_item_events (match_id, platform, patch, queue_id, timestamp_ms, participant_id, event_type, item_id, before_id, after_id) VALUES `, `?, ?, ?, ?, ?, ?, ?, ?, ?, ?`, len(rows), func(i int) []any {
		row := rows[i]
		return []any{row.MatchID, row.Platform, row.Patch, row.QueueID, row.TimestampMS, row.ParticipantID, row.EventType, row.ItemID, row.BeforeID, row.AfterID}
	})
}

func (r *Repository) insertTimelineSkillEvents(ctx context.Context, rows []analytics.TimelineSkillEventRow) error {
	return insertRowsInBatches(ctx, r.db, `INSERT INTO timeline_skill_events (match_id, platform, patch, queue_id, timestamp_ms, participant_id, skill_slot, skill_order, level_up_type) VALUES `, `?, ?, ?, ?, ?, ?, ?, ?, ?`, len(rows), func(i int) []any {
		row := rows[i]
		return []any{row.MatchID, row.Platform, row.Patch, row.QueueID, row.TimestampMS, row.ParticipantID, row.SkillSlot, row.SkillOrder, row.LevelUpType}
	})
}

func (r *Repository) insertTimelineCombatEvents(ctx context.Context, rows []analytics.TimelineCombatEventRow) error {
	return insertRowsInBatches(ctx, r.db, `INSERT INTO timeline_combat_events (match_id, platform, patch, queue_id, timestamp_ms, killer_id, victim_id, assisting_participant_ids, bounty, shutdown_bounty, position_x, position_y) VALUES `, `?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?`, len(rows), func(i int) []any {
		row := rows[i]
		return []any{row.MatchID, row.Platform, row.Patch, row.QueueID, row.TimestampMS, row.KillerID, row.VictimID, row.AssistingParticipantIDs, row.Bounty, row.ShutdownBounty, row.PositionX, row.PositionY}
	})
}

func (r *Repository) insertTimelineObjectiveEvents(ctx context.Context, rows []analytics.TimelineObjectiveEventRow) error {
	return insertRowsInBatches(ctx, r.db, `INSERT INTO timeline_objective_events (match_id, platform, patch, queue_id, timestamp_ms, event_type, killer_id, team_id, monster_type, monster_sub_type, building_type, tower_type, lane_type, position_x, position_y) VALUES `, `?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?`, len(rows), func(i int) []any {
		row := rows[i]
		return []any{row.MatchID, row.Platform, row.Patch, row.QueueID, row.TimestampMS, row.EventType, row.KillerID, row.TeamID, row.MonsterType, row.MonsterSubType, row.BuildingType, row.TowerType, row.LaneType, row.PositionX, row.PositionY}
	})
}

func (r *Repository) insertChampionBans(ctx context.Context, rows []analytics.ChampionBanRow) error {
	return insertRowsInBatches(ctx, r.db, `INSERT INTO champion_bans (match_id, platform, patch, queue_id, team_id, champion_id, pick_turn) VALUES `, `?, ?, ?, ?, ?, ?, ?`, len(rows), func(i int) []any {
		row := rows[i]
		return []any{row.MatchID, row.Platform, row.Patch, row.QueueID, row.TeamID, row.ChampionID, row.PickTurn}
	})
}

type execer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func insertRowsInBatches(ctx context.Context, db execer, insertPrefix, placeholders string, rowCount int, argsForRow func(int) []any) error {
	if rowCount == 0 {
		return nil
	}
	for start := 0; start < rowCount; start += maxBatchInsertRows {
		end := start + maxBatchInsertRows
		if end > rowCount {
			end = rowCount
		}
		if err := insertRows(ctx, db, insertPrefix, placeholders, start, end, argsForRow); err != nil {
			return err
		}
	}
	return nil
}

func insertRows(ctx context.Context, db execer, insertPrefix, placeholders string, start, end int, argsForRow func(int) []any) error {
	rowCount := end - start
	values := make([]string, rowCount)
	args := []any{}
	for i := 0; i < rowCount; i++ {
		values[i] = "(" + placeholders + ")"
		args = append(args, argsForRow(start+i)...)
	}
	_, err := db.ExecContext(ctx, insertPrefix+strings.Join(values, ", "), args...)
	return err
}

const participantColumns = `match_id, platform, patch, queue_id, participant_id, puuid, team_id, champion_id, champion_name, role, win, kills, deaths, assists, gold_earned, gold_spent, total_minions_killed, neutral_minions_killed, total_damage_dealt_to_champions, physical_damage_dealt_to_champions, magic_damage_dealt_to_champions, true_damage_dealt_to_champions, total_damage_taken, damage_self_mitigated, damage_dealt_to_objectives, damage_dealt_to_turrets, damage_dealt_to_buildings, vision_score, wards_placed, wards_killed, detector_wards_placed, time_ccing_others, total_heal, total_heals_on_teammates, total_damage_shielded_on_teammates, turret_takedowns, inhibitor_takedowns, dragon_kills, baron_kills, objectives_stolen, total_time_spent_dead, time_played, item0, item1, item2, item3, item4, item5, trinket_item, summoner_spell1, summoner_spell2, primary_rune_tree, secondary_rune_tree, keystone, rune_signature, spell_signature, final_items_signature, core2_signature, core3_signature, rank_bucket`

const participantPlaceholders = `?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?`

const participantPerformanceColumns = `match_id, platform, patch, queue_id, participant_id, champion_id, role, gold_earned, gold_spent, total_minions_killed, neutral_minions_killed, total_damage_dealt_to_champions, physical_damage_dealt_to_champions, magic_damage_dealt_to_champions, true_damage_dealt_to_champions, total_damage_taken, damage_self_mitigated, damage_dealt_to_objectives, damage_dealt_to_turrets, damage_dealt_to_buildings, vision_score, wards_placed, wards_killed, detector_wards_placed, time_ccing_others, total_heal, total_heals_on_teammates, total_damage_shielded_on_teammates, turret_takedowns, inhibitor_takedowns, dragon_kills, baron_kills, objectives_stolen, total_time_spent_dead, time_played`

const participantPerformancePlaceholders = `?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?`

func participantArgs(row analytics.ParticipantRow) []any {
	return []any{row.MatchID, row.Platform, row.Patch, row.QueueID, row.ParticipantID, row.PUUID, row.TeamID, row.ChampionID, row.ChampionName, row.Role, row.Win, row.Kills, row.Deaths, row.Assists, row.GoldEarned, row.GoldSpent, row.TotalMinionsKilled, row.NeutralMinionsKilled, row.TotalDamageDealtToChampions, row.PhysicalDamageDealtToChampions, row.MagicDamageDealtToChampions, row.TrueDamageDealtToChampions, row.TotalDamageTaken, row.DamageSelfMitigated, row.DamageDealtToObjectives, row.DamageDealtToTurrets, row.DamageDealtToBuildings, row.VisionScore, row.WardsPlaced, row.WardsKilled, row.DetectorWardsPlaced, row.TimeCCingOthers, row.TotalHeal, row.TotalHealsOnTeammates, row.TotalDamageShieldedOnTeammates, row.TurretTakedowns, row.InhibitorTakedowns, row.DragonKills, row.BaronKills, row.ObjectivesStolen, row.TotalTimeSpentDead, row.TimePlayed, row.Item0, row.Item1, row.Item2, row.Item3, row.Item4, row.Item5, row.TrinketItem, row.SummonerSpell1, row.SummonerSpell2, row.PrimaryRuneTree, row.SecondaryRuneTree, row.Keystone, row.RuneSignature, row.SpellSignature, row.FinalItemsSignature, row.Core2Signature, row.Core3Signature, row.RankBucket}
}

func participantPerformanceArgs(row analytics.ParticipantRow) []any {
	return []any{
		row.MatchID,
		row.Platform,
		row.Patch,
		row.QueueID,
		row.ParticipantID,
		row.ChampionID,
		row.Role,
		row.GoldEarned,
		row.GoldSpent,
		row.TotalMinionsKilled,
		row.NeutralMinionsKilled,
		row.TotalDamageDealtToChampions,
		row.PhysicalDamageDealtToChampions,
		row.MagicDamageDealtToChampions,
		row.TrueDamageDealtToChampions,
		row.TotalDamageTaken,
		row.DamageSelfMitigated,
		row.DamageDealtToObjectives,
		row.DamageDealtToTurrets,
		row.DamageDealtToBuildings,
		row.VisionScore,
		row.WardsPlaced,
		row.WardsKilled,
		row.DetectorWardsPlaced,
		row.TimeCCingOthers,
		row.TotalHeal,
		row.TotalHealsOnTeammates,
		row.TotalDamageShieldedOnTeammates,
		row.TurretTakedowns,
		row.InhibitorTakedowns,
		row.DragonKills,
		row.BaronKills,
		row.ObjectivesStolen,
		row.TotalTimeSpentDead,
		row.TimePlayed,
	}
}
