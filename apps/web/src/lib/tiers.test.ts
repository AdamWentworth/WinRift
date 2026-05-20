import { describe, expect, it } from 'vitest';
import { championTier, sPlusCutoff } from './tiers';

describe('champion tiers', () => {
  it('uses broader role-aware S+ bands', () => {
    expect(sPlusCutoff({ role: 'TOP', roleRankTotal: 50 })).toBe(5);
    expect(sPlusCutoff({ role: 'JUNGLE', roleRankTotal: 50 })).toBe(6);
    expect(sPlusCutoff({ role: 'MIDDLE', roleRankTotal: 50 })).toBe(8);
    expect(sPlusCutoff({ role: 'BOTTOM', roleRankTotal: 50 })).toBe(2);
    expect(sPlusCutoff({ role: 'UTILITY', roleRankTotal: 50 })).toBe(9);
  });

  it('scales S+ down for thin stored samples', () => {
    expect(sPlusCutoff({ role: 'UTILITY', roleRankTotal: 8 })).toBe(2);
    expect(championTier({ games: 20, role: 'UTILITY', roleRank: 2, roleRankTotal: 8, winRate: 52 })).toBe('S+');
    expect(championTier({ games: 20, role: 'UTILITY', roleRank: 3, roleRankTotal: 8, winRate: 52 })).toBe('A');
  });
});
