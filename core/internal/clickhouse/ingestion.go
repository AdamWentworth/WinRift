package clickhouse

import (
	"context"

	"winrift/core/internal/analytics"
)

func (r *Repository) InsertNormalized(ctx context.Context, normalized analytics.NormalizedMatch) error {
	if err := r.insertRawMatch(ctx, normalized.RawMatch); err != nil {
		return err
	}
	if err := r.insertRawTimeline(ctx, normalized.RawTimeline); err != nil {
		return err
	}
	for _, row := range normalized.Participants {
		if err := r.insertParticipant(ctx, row); err != nil {
			return err
		}
		if err := r.insertParticipantPerformance(ctx, row); err != nil {
			return err
		}
	}
	for _, row := range normalized.Matchups {
		if err := r.insertMatchup(ctx, row); err != nil {
			return err
		}
	}
	for _, row := range normalized.TimelineParticipantFrames {
		if err := r.insertTimelineParticipantFrame(ctx, row); err != nil {
			return err
		}
	}
	for _, row := range normalized.TimelineItemEvents {
		if err := r.insertTimelineItemEvent(ctx, row); err != nil {
			return err
		}
	}
	for _, row := range normalized.TimelineSkillEvents {
		if err := r.insertTimelineSkillEvent(ctx, row); err != nil {
			return err
		}
	}
	for _, row := range normalized.TimelineCombatEvents {
		if err := r.insertTimelineCombatEvent(ctx, row); err != nil {
			return err
		}
	}
	for _, row := range normalized.TimelineObjectiveEvents {
		if err := r.insertTimelineObjectiveEvent(ctx, row); err != nil {
			return err
		}
	}
	for _, row := range normalized.ChampionBans {
		if err := r.insertChampionBan(ctx, row); err != nil {
			return err
		}
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

func (r *Repository) insertParticipant(ctx context.Context, row analytics.ParticipantRow) error {
	_, err := r.db.ExecContext(ctx, participantInsertSQL, participantArgs(row)...)
	return err
}

func (r *Repository) insertParticipantPerformance(ctx context.Context, row analytics.ParticipantRow) error {
	_, err := r.db.ExecContext(ctx, participantPerformanceInsertSQL, participantPerformanceArgs(row)...)
	return err
}

func (r *Repository) insertMatchup(ctx context.Context, row analytics.MatchupRow) error {
	args := participantArgs(row.ParticipantRow)
	args = append(args, row.OpponentParticipantID, row.OpponentChampionID, row.OpponentRole)
	_, err := r.db.ExecContext(ctx, matchupInsertSQL, args...)
	return err
}

func (r *Repository) insertTimelineParticipantFrame(ctx context.Context, row analytics.TimelineParticipantFrameRow) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO timeline_participant_frames (match_id, platform, patch, queue_id, timestamp_ms, participant_id, level, xp, current_gold, total_gold, minions_killed, jungle_minions_killed, position_x, position_y, total_damage_done_to_champions, total_damage_taken) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.MatchID, row.Platform, row.Patch, row.QueueID, row.TimestampMS, row.ParticipantID, row.Level, row.XP, row.CurrentGold, row.TotalGold, row.MinionsKilled, row.JungleMinionsKilled, row.PositionX, row.PositionY, row.TotalDamageDoneToChampions, row.TotalDamageTaken,
	)
	return err
}

func (r *Repository) insertTimelineItemEvent(ctx context.Context, row analytics.TimelineItemEventRow) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO timeline_item_events (match_id, platform, patch, queue_id, timestamp_ms, participant_id, event_type, item_id, before_id, after_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.MatchID, row.Platform, row.Patch, row.QueueID, row.TimestampMS, row.ParticipantID, row.EventType, row.ItemID, row.BeforeID, row.AfterID,
	)
	return err
}

func (r *Repository) insertTimelineSkillEvent(ctx context.Context, row analytics.TimelineSkillEventRow) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO timeline_skill_events (match_id, platform, patch, queue_id, timestamp_ms, participant_id, skill_slot, skill_order, level_up_type) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.MatchID, row.Platform, row.Patch, row.QueueID, row.TimestampMS, row.ParticipantID, row.SkillSlot, row.SkillOrder, row.LevelUpType,
	)
	return err
}

func (r *Repository) insertTimelineCombatEvent(ctx context.Context, row analytics.TimelineCombatEventRow) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO timeline_combat_events (match_id, platform, patch, queue_id, timestamp_ms, killer_id, victim_id, assisting_participant_ids, bounty, shutdown_bounty, position_x, position_y) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.MatchID, row.Platform, row.Patch, row.QueueID, row.TimestampMS, row.KillerID, row.VictimID, row.AssistingParticipantIDs, row.Bounty, row.ShutdownBounty, row.PositionX, row.PositionY,
	)
	return err
}

func (r *Repository) insertTimelineObjectiveEvent(ctx context.Context, row analytics.TimelineObjectiveEventRow) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO timeline_objective_events (match_id, platform, patch, queue_id, timestamp_ms, event_type, killer_id, team_id, monster_type, monster_sub_type, building_type, tower_type, lane_type, position_x, position_y) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.MatchID, row.Platform, row.Patch, row.QueueID, row.TimestampMS, row.EventType, row.KillerID, row.TeamID, row.MonsterType, row.MonsterSubType, row.BuildingType, row.TowerType, row.LaneType, row.PositionX, row.PositionY,
	)
	return err
}

func (r *Repository) insertChampionBan(ctx context.Context, row analytics.ChampionBanRow) error {
	_, err := r.db.ExecContext(
		ctx,
		`INSERT INTO champion_bans (match_id, platform, patch, queue_id, team_id, champion_id, pick_turn) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		row.MatchID, row.Platform, row.Patch, row.QueueID, row.TeamID, row.ChampionID, row.PickTurn,
	)
	return err
}

const participantColumns = `match_id, platform, patch, queue_id, participant_id, puuid, team_id, champion_id, champion_name, role, win, kills, deaths, assists, gold_earned, gold_spent, total_minions_killed, neutral_minions_killed, total_damage_dealt_to_champions, physical_damage_dealt_to_champions, magic_damage_dealt_to_champions, true_damage_dealt_to_champions, total_damage_taken, damage_self_mitigated, damage_dealt_to_objectives, damage_dealt_to_turrets, damage_dealt_to_buildings, vision_score, wards_placed, wards_killed, detector_wards_placed, time_ccing_others, total_heal, total_heals_on_teammates, total_damage_shielded_on_teammates, turret_takedowns, inhibitor_takedowns, dragon_kills, baron_kills, objectives_stolen, total_time_spent_dead, time_played, item0, item1, item2, item3, item4, item5, trinket_item, summoner_spell1, summoner_spell2, primary_rune_tree, secondary_rune_tree, keystone, rune_signature, spell_signature, final_items_signature, core2_signature, core3_signature, rank_bucket`

const participantPlaceholders = `?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?`

const participantInsertSQL = `INSERT INTO participants (` + participantColumns + `) VALUES (` + participantPlaceholders + `)`

const matchupInsertSQL = `INSERT INTO participant_matchups (` + participantColumns + `, opponent_participant_id, opponent_champion_id, opponent_role) VALUES (` + participantPlaceholders + `, ?, ?, ?)`

const participantPerformanceColumns = `match_id, platform, patch, queue_id, participant_id, champion_id, role, gold_earned, gold_spent, total_minions_killed, neutral_minions_killed, total_damage_dealt_to_champions, physical_damage_dealt_to_champions, magic_damage_dealt_to_champions, true_damage_dealt_to_champions, total_damage_taken, damage_self_mitigated, damage_dealt_to_objectives, damage_dealt_to_turrets, damage_dealt_to_buildings, vision_score, wards_placed, wards_killed, detector_wards_placed, time_ccing_others, total_heal, total_heals_on_teammates, total_damage_shielded_on_teammates, turret_takedowns, inhibitor_takedowns, dragon_kills, baron_kills, objectives_stolen, total_time_spent_dead, time_played`

const participantPerformancePlaceholders = `?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?`

const participantPerformanceInsertSQL = `INSERT INTO participant_performance (` + participantPerformanceColumns + `) VALUES (` + participantPerformancePlaceholders + `)`

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
