import type { ChampionGuideSummary } from '../api/types';

type TierableSummary = Pick<ChampionGuideSummary, 'games' | 'roleRank' | 'roleRankTotal' | 'tierScore' | 'winRate'>;

export function championTier(summary?: TierableSummary) {
	if (!summary?.games) return '?';
	if (isSPlus(summary)) return 'S+';
	if (typeof summary.tierScore === 'number') {
		return capTierByWinRate(summary, tierFromScore(summary.tierScore));
	}
	if (summary.roleRank && summary.roleRankTotal) {
		const percentile = summary.roleRank / summary.roleRankTotal;
		if (percentile <= 0.22) return capTierByWinRate(summary, 'S');
		if (percentile <= 0.4) return capTierByWinRate(summary, 'A');
		if (percentile <= 0.65) return capTierByWinRate(summary, 'B');
		if (percentile <= 0.85) return capTierByWinRate(summary, 'C');
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
		return summary.tierScore >= 59 && summary.winRate >= 50;
	}
	return Boolean(summary.roleRank && summary.roleRankTotal && summary.roleRank / summary.roleRankTotal <= 0.05 && summary.winRate >= 50);
}

function tierFromScore(score: number) {
	if (score >= 56) return 'S';
	if (score >= 52) return 'A';
	if (score >= 48) return 'B';
	if (score >= 44) return 'C';
	return 'D';
}

function capTierByWinRate(summary: TierableSummary, tier: string) {
	if (summary.winRate < 47 && ['S', 'A'].includes(tier)) return 'B';
	if (summary.winRate < 49 && tier === 'S') return 'A';
	return tier;
}
