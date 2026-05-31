package clickhouse

import (
	"context"
	"fmt"
	"strings"

	"winrift/core/internal/analytics"
)

type ParticipantPerformanceBackfillResult struct {
	Rows int
}

func (r *Repository) BackfillParticipantPerformance(ctx context.Context, patch string, queueID uint16) (ParticipantPerformanceBackfillResult, error) {
	patch = strings.TrimSpace(patch)
	if patch == "" {
		return ParticipantPerformanceBackfillResult{}, fmt.Errorf("patch is required")
	}
	if queueID == 0 {
		queueID = analytics.RankedSoloQueueID
	}
	if _, err := r.db.ExecContext(ctx, `ALTER TABLE participant_performance DELETE WHERE patch = ? AND queue_id = ? SETTINGS mutations_sync = 2`, patch, queueID); err != nil {
		return ParticipantPerformanceBackfillResult{}, err
	}
	platforms, err := r.rawMatchPlatforms(ctx, patch, queueID)
	if err != nil {
		return ParticipantPerformanceBackfillResult{}, err
	}
	for _, platform := range platforms {
		if err := r.backfillParticipantPerformanceForPlatform(ctx, patch, platform, queueID); err != nil {
			return ParticipantPerformanceBackfillResult{}, err
		}
	}
	var result ParticipantPerformanceBackfillResult
	err = r.db.QueryRowContext(ctx, `SELECT count() FROM participant_performance WHERE patch = ? AND queue_id = ?`, patch, queueID).Scan(&result.Rows)
	return result, err
}

func (r *Repository) backfillParticipantPerformanceForPlatform(ctx context.Context, patch, platform string, queueID uint16) error {
	_, err := r.db.ExecContext(ctx, fmt.Sprintf(`
		INSERT INTO participant_performance
		(%s)
		SELECT
			match_id,
			platform,
			patch,
			queue_id,
			participant_id,
			champion_id,
			role,
			toUInt32(greatest(JSONExtractInt(participant, 'goldEarned'), 0)) AS gold_earned,
			toUInt32(greatest(JSONExtractInt(participant, 'goldSpent'), 0)) AS gold_spent,
			toUInt32(greatest(JSONExtractInt(participant, 'totalMinionsKilled'), 0)) AS total_minions_killed,
			toUInt32(greatest(JSONExtractInt(participant, 'neutralMinionsKilled'), 0)) AS neutral_minions_killed,
			toUInt32(greatest(JSONExtractInt(participant, 'totalDamageDealtToChampions'), 0)) AS total_damage_dealt_to_champions,
			toUInt32(greatest(JSONExtractInt(participant, 'physicalDamageDealtToChampions'), 0)) AS physical_damage_dealt_to_champions,
			toUInt32(greatest(JSONExtractInt(participant, 'magicDamageDealtToChampions'), 0)) AS magic_damage_dealt_to_champions,
			toUInt32(greatest(JSONExtractInt(participant, 'trueDamageDealtToChampions'), 0)) AS true_damage_dealt_to_champions,
			toUInt32(greatest(JSONExtractInt(participant, 'totalDamageTaken'), 0)) AS total_damage_taken,
			toUInt32(greatest(JSONExtractInt(participant, 'damageSelfMitigated'), 0)) AS damage_self_mitigated,
			toUInt32(greatest(JSONExtractInt(participant, 'damageDealtToObjectives'), 0)) AS damage_dealt_to_objectives,
			toUInt32(greatest(JSONExtractInt(participant, 'damageDealtToTurrets'), 0)) AS damage_dealt_to_turrets,
			toUInt32(greatest(JSONExtractInt(participant, 'damageDealtToBuildings'), 0)) AS damage_dealt_to_buildings,
			toUInt32(greatest(JSONExtractInt(participant, 'visionScore'), 0)) AS vision_score,
			toUInt32(greatest(JSONExtractInt(participant, 'wardsPlaced'), 0)) AS wards_placed,
			toUInt32(greatest(JSONExtractInt(participant, 'wardsKilled'), 0)) AS wards_killed,
			toUInt32(greatest(JSONExtractInt(participant, 'detectorWardsPlaced'), 0)) AS detector_wards_placed,
			toUInt32(greatest(JSONExtractInt(participant, 'timeCCingOthers'), 0)) AS time_ccing_others,
			toUInt32(greatest(JSONExtractInt(participant, 'totalHeal'), 0)) AS total_heal,
			toUInt32(greatest(JSONExtractInt(participant, 'totalHealsOnTeammates'), 0)) AS total_heals_on_teammates,
			toUInt32(greatest(JSONExtractInt(participant, 'totalDamageShieldedOnTeammates'), 0)) AS total_damage_shielded_on_teammates,
			toUInt32(greatest(JSONExtractInt(participant, 'turretTakedowns'), 0)) AS turret_takedowns,
			toUInt32(greatest(JSONExtractInt(participant, 'inhibitorTakedowns'), 0)) AS inhibitor_takedowns,
			toUInt32(greatest(JSONExtractInt(participant, 'dragonKills'), 0)) AS dragon_kills,
			toUInt32(greatest(JSONExtractInt(participant, 'baronKills'), 0)) AS baron_kills,
			toUInt32(greatest(JSONExtractInt(participant, 'objectivesStolen'), 0)) AS objectives_stolen,
			toUInt32(greatest(JSONExtractInt(participant, 'totalTimeSpentDead'), 0)) AS total_time_spent_dead,
			toUInt32(multiIf(
				JSONExtractInt(participant, 'timePlayed') > 0,
				JSONExtractInt(participant, 'timePlayed'),
				toInt64(duration_seconds)
			)) AS time_played
		FROM
		(
			SELECT
				match_id,
				platform,
				patch,
				queue_id,
				duration_seconds,
				toUInt8(JSONExtractInt(participant, 'participantId')) AS participant_id,
				toUInt16(JSONExtractInt(participant, 'championId')) AS champion_id,
				multiIf(
					JSONExtractString(participant, 'teamPosition') != '',
					JSONExtractString(participant, 'teamPosition'),
					JSONExtractString(participant, 'individualPosition')
				) AS role,
				participant
			FROM raw_matches AS rm FINAL
			ARRAY JOIN JSONExtractArrayRaw(JSONExtractRaw(raw_json, 'info'), 'participants') AS participant
			WHERE patch = ?
				AND platform = ?
				AND queue_id = ?
		)
		WHERE participant_id > 0`, participantPerformanceColumns),
		patch,
		platform,
		queueID,
	)
	return err
}
