import { describe, expect, it } from 'vitest';
import { championTier, isSPlus } from './tiers';

describe('champion tiers', () => {
  it('awards S+ from score threshold instead of role quotas', () => {
    expect(isSPlus({ confidence: 45, games: 80, roleRank: 14, roleRankTotal: 80, tierScore: 72, winRate: 51 })).toBe(true);
    expect(championTier({ confidence: 45, games: 80, roleRank: 14, roleRankTotal: 80, tierScore: 72, winRate: 51 })).toBe('S+');
  });

  it('awards S+ from strong winrate with enough confidence', () => {
    expect(championTier({ confidence: 48, games: 100, roleRank: 9, roleRankTotal: 100, winRate: 54 })).toBe('S+');
    expect(championTier({ confidence: 47.99, games: 100, roleRank: 9, roleRankTotal: 100, winRate: 54 })).toBe('S');
    expect(championTier({ confidence: 48, games: 99, roleRank: 9, roleRankTotal: 100, winRate: 54 })).toBe('S');
  });
});
