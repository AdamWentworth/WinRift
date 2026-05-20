import type { ChampionGuideSummary } from '../api/types';

type TierableSummary = Pick<ChampionGuideSummary, 'games' | 'role' | 'roleRank' | 'roleRankTotal' | 'winRate'>;

const sPlusRoleCaps: Record<string, number> = {
  TOP: 5,
  JUNGLE: 6,
  MIDDLE: 8,
  BOTTOM: 2,
  UTILITY: 9,
  ALL: 12,
};

export function championTier(summary?: TierableSummary) {
  if (!summary?.games) return '?';
  if (summary.roleRank && summary.roleRankTotal) {
    if (summary.roleRank <= sPlusCutoff(summary)) return 'S+';
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

export function sPlusCutoff(summary: Pick<TierableSummary, 'role' | 'roleRankTotal'>) {
  const role = normalizeTierRole(summary.role);
  const cap = sPlusRoleCaps[role] ?? sPlusRoleCaps.ALL;
  const sampleScaledCap = Math.max(1, Math.ceil(summary.roleRankTotal * 0.18));
  return Math.min(cap, sampleScaledCap);
}

function normalizeTierRole(role: string) {
  const normalized = role.toUpperCase();
  if (normalized === 'SUPPORT') return 'UTILITY';
  return normalized || 'ALL';
}
