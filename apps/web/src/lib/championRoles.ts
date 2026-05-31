import type { ChampionRoleRate } from '../api/types';
import { normalizeRole } from './roles';

export function itemContextForRole(role: string): 'JUNGLE' | 'SUPPORT' | undefined {
  if (role === 'JUNGLE') return 'JUNGLE';
  if (role === 'UTILITY') return 'SUPPORT';
  return undefined;
}

export function mainChampionRole(rows: ChampionRoleRate[], championId: number) {
  const ranked = rows
    .filter((row) => row.championId === championId && normalizeRole(row.role))
    .sort((a, b) => {
      if (a.games !== b.games) return b.games - a.games;
      return b.pickRate - a.pickRate;
    });
  return normalizeRole(ranked[0]?.role);
}
