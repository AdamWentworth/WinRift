import { describe, expect, it } from 'vitest';
import { selectGuideItemPanelRows, selectStartingLoadoutRows } from './BuildGuidePage';
import type { GuideItemSlot, GuideStartingLoadout } from './BuildGuidePage';

function slot(itemSlot: number, itemId: number, games: number, winRate = 55, confidence = 50): GuideItemSlot {
  return {
    itemSlot,
    itemId,
    wins: Math.round((winRate / 100) * games),
    games,
    winRate,
    confidence,
  };
}

function loadout(itemSignature: string, games: number, winRate = 55, confidence = 50): GuideStartingLoadout {
  return {
    itemSignature,
    wins: Math.round((winRate / 100) * games),
    games,
    winRate,
    confidence,
  };
}

describe('selectGuideItemPanelRows', () => {
  it('dedupes starting items and late build options for player-readable builds', () => {
    const panels = selectGuideItemPanelRows([
      slot(0, 1056, 100, 51, 40),
      slot(0, 1056, 90, 53, 41),
      slot(0, 1082, 80, 54, 39),
      slot(1, 3115, 300, 52, 50),
      slot(2, 3100, 250, 53, 51),
      slot(3, 3089, 200, 54, 52),
      slot(4, 3157, 160, 60, 55),
      slot(4, 3089, 150, 58, 54),
      slot(5, 3157, 140, 61, 56),
      slot(5, 3135, 130, 57, 53),
      slot(6, 3135, 100, 59, 51),
      slot(6, 4645, 90, 58, 50),
    ]);

    expect(panels.startingRows.map((row) => row.itemId)).toEqual([1056, 1082]);
    expect(panels.coreRows.map((row) => row.itemId)).toEqual([3115, 3100, 3089]);
    expect(panels.fourthRows.map((row) => row.itemId)).toContain(3157);
    expect(panels.fifthRows.map((row) => row.itemId)).not.toContain(3157);
    expect(panels.sixthRows.map((row) => row.itemId)).not.toContain(3135);
  });

  it('keeps starting item bundles with potion quantities intact', () => {
    const rows = selectStartingLoadoutRows([
      loadout('1056-2003-2003', 80, 52, 45),
      loadout('1056-2003-2003', 75, 53, 44),
      loadout('1056-2055', 24, 57, 40),
    ]);

    expect(rows.map((row) => row.itemSignature)).toEqual(['1056-2003-2003', '1056-2055']);
  });
});
