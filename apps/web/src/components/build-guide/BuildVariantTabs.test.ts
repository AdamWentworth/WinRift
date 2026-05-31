import { describe, expect, it } from 'vitest';
import type { ChampionGuideBuildVariant } from '../../api/types';
import { buildVariantLabel, groupBuildVariantsForDisplay } from './BuildVariantTabs';

function variant(overrides: Partial<ChampionGuideBuildVariant> & Pick<ChampionGuideBuildVariant, 'variantKey'>): ChampionGuideBuildVariant {
  const games = overrides.games ?? 10;
  const wins = overrides.wins ?? Math.round(games / 2);
  return {
    variantKey: overrides.variantKey,
    variantLabel: overrides.variantLabel,
    variantTags: overrides.variantTags ?? [],
    core2Signature: overrides.core2Signature ?? '100-101',
    core3Signature: overrides.core3Signature ?? '100-101-102',
    finalItemsSignature: overrides.finalItemsSignature ?? '100-101-102-103-104-105',
    runeSignature: overrides.runeSignature ?? '8000|8100|8010-9111-9104-8299|5005-5008-5011',
    spellSignature: overrides.spellSignature ?? '4-14',
    skillOrderSignature: overrides.skillOrderSignature,
    skillOrderWins: overrides.skillOrderWins,
    skillOrderGames: overrides.skillOrderGames,
    skillOrderWinRate: overrides.skillOrderWinRate,
    skillOrderConfidence: overrides.skillOrderConfidence,
    wins,
    games,
    winRate: overrides.winRate ?? (games ? (wins / games) * 100 : 0),
    confidence: overrides.confidence ?? 45,
    buildCount: overrides.buildCount ?? 1,
  };
}

describe('groupBuildVariantsForDisplay', () => {
  it('merges repeated backend labels into one player-facing build tab', () => {
    const grouped = groupBuildVariantsForDisplay([
      variant({
        variantKey: 'ap-burst-1',
        variantLabel: 'AP Burst',
        variantTags: ['burst'],
        core2Signature: '3100-3089',
        games: 120,
        wins: 66,
        confidence: 55,
        buildCount: 3,
      }),
      variant({
        variantKey: 'ap-burst-2',
        variantLabel: 'AP Burst',
        variantTags: ['reset'],
        core2Signature: '4636-3089',
        games: 80,
        wins: 44,
        confidence: 48,
        buildCount: 2,
        skillOrderSignature: 'Q-E-W',
        skillOrderGames: 30,
      }),
      variant({
        variantKey: 'on-hit-1',
        variantLabel: 'On Hit',
        core2Signature: '3124-3153',
        games: 90,
        wins: 45,
      }),
    ]);

    expect(grouped).toHaveLength(2);
    const apBurst = grouped.find((candidate) => candidate.variantLabel === 'AP Burst');
    expect(apBurst).toBeTruthy();
    expect(apBurst?.variantKey).toBe('label:ap-burst');
    expect(apBurst?.games).toBe(200);
    expect(apBurst?.wins).toBe(110);
    expect(apBurst?.winRate).toBeCloseTo(55);
    expect(apBurst?.buildCount).toBe(5);
    expect(apBurst?.variantTags).toEqual(['burst', 'reset']);
    expect(apBurst?.core2Signature).toBe('3100-3089');
    expect(apBurst?.skillOrderSignature).toBe('Q-E-W');
    expect(buildVariantLabel(apBurst as ChampionGuideBuildVariant, 0)).toBe('AP Burst');
  });
});
