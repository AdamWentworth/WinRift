import type { ChampionGuideSummary } from '../api/types';

type TierableSummary = Pick<ChampionGuideSummary, 'games' | 'roleRank' | 'roleRankTotal' | 'tierScore' | 'winRate'>;

export function championTier(summary?: TierableSummary) {
	if (!summary?.games) return '?';
	if (isSPlus(summary)) return 'S+';
	if (summary.roleRank && summary.roleRankTotal) {
		const percentile = summary.roleRank / summary.roleRankTotal;
		if (percentile <= 0.22) return 'S';
		if (percentile <= 0.4) return 'A';
		if (percentile <= 0.65) return 'B';
		if (percentile <= 0.85) return 'C';
		return 'D';
	}
	if (summary.games >= 250 && summary.winRate >= 53) return 'S';
	if (summary.games >= 100 && summary.winRate >= 51.5) return 'A';
	if (summary.winRate >= 50) return 'B';
	if (summary.winRate >= 48) return 'C';
	return 'D';
}

export function isSPlus(summary: TierableSummary) {
	if (typeof summary.tierScore === 'number') {
		return summary.tierScore >= 59;
	}
	return Boolean(summary.roleRank && summary.roleRankTotal && summary.roleRank / summary.roleRankTotal <= 0.05);
}
