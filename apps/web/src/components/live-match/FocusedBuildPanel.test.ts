import { describe, expect, it } from 'vitest';
import type { AnalyticsItemSlot, ItemData } from '../../api/types';
import { selectBuildSlotRows } from './FocusedBuildPanel';

const items = {
  version: 'test',
  data: {
    data: {
      3047: { name: 'Plated Steelcaps', tags: ['Armor', 'Boots'] },
      3111: { name: "Mercury's Treads", tags: ['Boots', 'SpellBlock'] },
      3026: { name: 'Guardian Angel', tags: ['Armor'] },
      3053: { name: "Sterak's Gage", tags: ['Health'] },
    },
  },
} satisfies ItemData;

function slot(row: Partial<AnalyticsItemSlot> & Pick<AnalyticsItemSlot, 'itemSlot' | 'itemId'>): AnalyticsItemSlot {
  return {
    championId: 421,
    role: 'JUNGLE',
    opponentChampionId: 950,
    patchBucket: '16.10',
    rankBucket: 'ALL',
    wins: 5,
    games: 10,
    winRate: 50,
    confidence: 50,
    ...row,
  };
}

describe('selectBuildSlotRows', () => {
  it('keeps only one boots item in the displayed slot read', () => {
    const selected = selectBuildSlotRows([
      slot({ itemSlot: 4, itemId: 3047, games: 5, winRate: 80, confidence: 45 }),
      slot({ itemSlot: 4, itemId: 3053, games: 12, winRate: 58, confidence: 40 }),
      slot({ itemSlot: 5, itemId: 3111, games: 34, winRate: 68, confidence: 58 }),
      slot({ itemSlot: 5, itemId: 3026, games: 20, winRate: 55, confidence: 42 }),
    ], items);

    expect(selected.find((row) => row.slot === 4)?.row?.itemId).toBe(3053);
    expect(selected.find((row) => row.slot === 5)?.row?.itemId).toBe(3111);
    expect(selected.flatMap((row) => row.row ? [row.row.itemId] : []).filter((itemId) => itemId === 3047 || itemId === 3111)).toHaveLength(1);
  });
});
