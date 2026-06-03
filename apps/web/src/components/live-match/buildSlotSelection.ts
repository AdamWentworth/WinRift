import type { AnalyticsItemSlot, ItemData } from '../../api/types';

export function selectBuildSlotRows(itemSlots: AnalyticsItemSlot[], items?: ItemData) {
  const displayedSlots = [0, 1, 2, 3, 4, 5, 6];
  const slotRows = displayedSlots.map((slot) => itemSlots.filter((candidate) => candidate.itemSlot === slot));
  const initialRows = slotRows.map((rows) => rows[0]).filter((row): row is AnalyticsItemSlot => Boolean(row));
  const selectedBoot = selectSingleBootRow(initialRows, items);
  return slotRows.map((rows, index) => {
    const slot = displayedSlots[index];
    const filteredRows = selectedBoot
      ? rows.filter((row) => !isBootItem(items, row.itemId) || sameItemSlotRow(row, selectedBoot))
      : rows;
    return {
      slot,
      row: filteredRows[0],
      commonRow: mostPlayedItemSlot(filteredRows),
    };
  });
}

function selectSingleBootRow(rows: AnalyticsItemSlot[], items?: ItemData) {
  const bootRows = rows.filter((row) => isBootItem(items, row.itemId));
  if (bootRows.length <= 1) return bootRows[0];
  return bootRows.reduce((best, row) => {
    if (row.confidence !== best.confidence) return row.confidence > best.confidence ? row : best;
    if (row.games !== best.games) return row.games > best.games ? row : best;
    if (row.winRate !== best.winRate) return row.winRate > best.winRate ? row : best;
    return row.itemSlot < best.itemSlot ? row : best;
  });
}

function sameItemSlotRow(a: AnalyticsItemSlot, b: AnalyticsItemSlot) {
  return a.itemSlot === b.itemSlot && a.itemId === b.itemId;
}

function isBootItem(items: ItemData | undefined, itemId: number) {
  const item = items?.data.data[String(itemId)];
  if (item?.tags?.some((tag) => tag.toLowerCase() === 'boots')) return true;
  return BOOT_ITEM_IDS.has(itemId);
}

function mostPlayedItemSlot(rows: AnalyticsItemSlot[]) {
  return rows.reduce<AnalyticsItemSlot | undefined>((best, row) => {
    if (!best) return row;
    if (row.games !== best.games) return row.games > best.games ? row : best;
    if (row.winRate !== best.winRate) return row.winRate > best.winRate ? row : best;
    return row.itemId < best.itemId ? row : best;
  }, undefined);
}

const BOOT_ITEM_IDS = new Set([
  1001,
  3006,
  3008,
  3009,
  3020,
  3047,
  3111,
  3117,
  3158,
  3171,
  223006,
  223008,
  223009,
  223020,
  223047,
  223111,
  223158,
]);
