import { describe, expect, it } from 'vitest';
import { championTier, isSPlus } from './tiers';

describe('champion tiers', () => {
	it('awards S+ from the composite score threshold', () => {
		expect(isSPlus({ games: 80, roleRank: 14, roleRankTotal: 80, tierScore: 59, winRate: 51 })).toBe(true);
		expect(championTier({ games: 80, roleRank: 14, roleRankTotal: 80, tierScore: 59, winRate: 51 })).toBe('S+');
	});

	it('does not award S+ from raw winrate alone', () => {
		expect(championTier({ games: 100, roleRank: 9, roleRankTotal: 100, tierScore: 58.99, winRate: 56 })).toBe('S');
	});

	it('does not award elite tiers to losing high-presence picks', () => {
		expect(isSPlus({ games: 1960, roleRank: 2, roleRankTotal: 148, tierScore: 59.2, winRate: 48.8 })).toBe(false);
		expect(championTier({ games: 1960, roleRank: 2, roleRankTotal: 148, tierScore: 58.9, winRate: 48.8 })).toBe('A');
	});

	it('falls back to top rank percentile when the backend score is missing', () => {
		expect(championTier({ games: 50, roleRank: 5, roleRankTotal: 100, winRate: 50 })).toBe('S+');
		expect(championTier({ games: 50, roleRank: 6, roleRankTotal: 100, winRate: 56 })).toBe('S');
	});
});
