import { describe, expect, it } from 'vitest';
import type { AnalyticsPatchStat } from '../api/types';
import { fallbackAnalyticsPatch, recommendedAnalyticsPatch } from './analyticsPatch';

function patch(patch: string, matches: number, current = false): AnalyticsPatchStat {
  return {
    patch,
    matches,
    participantSamples: matches * 10,
    rawMatches: matches,
    compiledMatches: 0,
    current,
  };
}

describe('recommendedAnalyticsPatch', () => {
  it('keeps the current patch selected while its sample is still filling', () => {
    const options = [
      patch('16.17', 2_426, true),
      patch('16.16', 62_495),
      patch('16.13', 350_705),
    ];

    expect(recommendedAnalyticsPatch(options, '16.17')).toBe('16.17');
  });

  it('uses the newest available patch when static metadata is ahead or unavailable', () => {
    const options = [
      patch('16.13', 350_705),
      patch('16.16', 62_495),
      patch('16.15', 13_898),
    ];

    expect(recommendedAnalyticsPatch(options, '16.17')).toBe('16.16');
  });
});

describe('fallbackAnalyticsPatch', () => {
  it('uses the newest mature previous patch for an empty current-patch guide', () => {
    const options = [
      patch('16.17', 2_426, true),
      patch('16.16', 62_495),
      patch('16.15', 3_898),
      patch('16.13', 350_705),
    ];

    expect(fallbackAnalyticsPatch(options, '16.17')).toBe('16.16');
  });

  it('uses the newest previous patch when no previous sample has reached maturity', () => {
    const options = [
      patch('16.17', 200, true),
      patch('16.16', 400),
      patch('16.15', 600),
    ];

    expect(fallbackAnalyticsPatch(options, '16.17')).toBe('16.16');
  });
});
