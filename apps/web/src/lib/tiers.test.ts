import { describe, expect, it } from 'vitest';
import { championTier, isSPlus } from './tiers';

describe('champion tiers', () => {
	it('awards S+ from the composite score threshold', () => {
		expect(isSPlus({ games: 80, roleRank: 14, roleRankTotal: 80, tierScore: 60, winRate: 51 })).toBe(true);
		expect(championTier({ games: 80, roleRank: 14, roleRankTotal: 80, tierScore: 60, winRate: 51 })).toBe('S+');
	});

	it('does not award S+ from raw winrate alone', () => {
		expect(championTier({ games: 100, roleRank: 9, roleRankTotal: 100, tierScore: 59.99, winRate: 56 })).toBe('S');
	});

	it('falls back to top rank percentile when the backend score is missing', () => {
		expect(championTier({ games: 50, roleRank: 5, roleRankTotal: 100, winRate: 50 })).toBe('S+');
		expect(championTier({ games: 50, roleRank: 6, roleRankTotal: 100, winRate: 56 })).toBe('S');
	});
});
