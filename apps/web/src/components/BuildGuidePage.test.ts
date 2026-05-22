import { describe, expect, it } from 'vitest';
import { selectGuideItemPanelRows } from './BuildGuidePage';
import type { GuideItemSlot } from './BuildGuidePage';

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
});
