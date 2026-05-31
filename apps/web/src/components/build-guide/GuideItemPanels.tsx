import type { AnalyticsItemSlot, ItemData, StartingItemLoadout } from '../../api/types';
import { itemImageUrl, itemName } from '../../lib/staticData';
import { EmptyState, PanelCard, PanelTitle } from '../ui/Panel';

export type GuideItemSlot = Pick<AnalyticsItemSlot, 'itemSlot' | 'itemId' | 'wins' | 'games' | 'winRate' | 'confidence'>;
export type GuideStartingLoadout = Pick<StartingItemLoadout, 'itemSignature' | 'wins' | 'games' | 'winRate' | 'confidence'>;

export function ItemGuideGrid({ rows, startingLoadouts, items, loading, context }: { rows: GuideItemSlot[]; startingLoadouts: GuideStartingLoadout[]; items?: ItemData; loading: boolean; context: string }) {
  const { startingRows, coreRows, fourthRows, fifthRows, sixthRows } = selectGuideItemPanelRows(rows);
  const startingOptions = selectStartingLoadoutRows(startingLoadouts);
  return (
    <section className="guide-item-grid">
      <GuideStartingItemPanel title="Starting Items" subtitle={context} loadouts={startingOptions} fallbackRows={startingRows.slice(0, 3)} items={items} loading={loading} />
      <GuideItemPanel title="Core Items" subtitle="Highest-confidence path" rows={coreRows} items={items} loading={loading} linked />
      <GuideItemPanel title="Fourth Item Options" subtitle="Options after core" rows={fourthRows} items={items} loading={loading} />
      <GuideItemPanel title="Fifth Item Options" subtitle="Late build pivots" rows={fifthRows} items={items} loading={loading} />
      <GuideItemPanel title="Sixth Item Options" subtitle="Full-build finishers" rows={sixthRows} items={items} loading={loading} />
    </section>
  );
}

export function selectStartingLoadoutRows(rows: GuideStartingLoadout[]) {
  const seen = new Set<string>();
  return [...rows]
    .filter((row) => {
      const signature = normalizedItemSignature(row.itemSignature);
      if (!signature || seen.has(signature)) return false;
      seen.add(signature);
      return true;
    })
    .sort((a, b) => {
      const leftScore = guideStartingLoadoutScore(a);
      const rightScore = guideStartingLoadoutScore(b);
      if (leftScore !== rightScore) return rightScore - leftScore;
      if (a.confidence !== b.confidence) return b.confidence - a.confidence;
      if (a.winRate !== b.winRate) return b.winRate - a.winRate;
      if (a.games !== b.games) return b.games - a.games;
      return normalizedItemSignature(a.itemSignature).localeCompare(normalizedItemSignature(b.itemSignature));
    })
    .slice(0, 3);
}

export function selectGuideItemPanelRows(rows: GuideItemSlot[]) {
  const startingRows = uniqueItemRows(sortedSlotCandidates(rows, 0)).slice(0, 3);
  const completedSlotRows = [1, 2, 3, 4, 5, 6].map((slot) => sortedSlotCandidates(rows, slot));
  const coreRows = pickUniqueCoreRows(completedSlotRows.slice(0, 3));
  const displayedLateItems = new Set(coreRows.map((row) => row.itemId));
  const fourthRows = optionRows(completedSlotRows[3] ?? [], displayedLateItems);
  addDisplayedItems(displayedLateItems, fourthRows);
  const fifthRows = optionRows(completedSlotRows[4] ?? [], displayedLateItems);
  addDisplayedItems(displayedLateItems, fifthRows);
  const sixthRows = optionRows(completedSlotRows[5] ?? [], displayedLateItems);
  return { startingRows, coreRows, fourthRows, fifthRows, sixthRows };
}

export function itemSignatureFromSlots(rows: GuideItemSlot[]) {
  const slotRows = [1, 2, 3, 4, 5, 6].map((slot) => sortedSlotCandidates(rows, slot));
  return pickUniqueCoreRows(slotRows.slice(0, 3)).map((row) => row.itemId).join('-');
}

function GuideItemPanel({ title, subtitle, rows, items, loading, linked }: { title: string; subtitle: string; rows: GuideItemSlot[]; items?: ItemData; loading: boolean; linked?: boolean }) {
  return (
    <PanelCard className="guide-card guide-item-panel">
      <PanelTitle title={title} />
      {rows.length ? (
        <div className={linked ? 'guide-item-list linked' : 'guide-item-list'}>
          {rows.map((row, index) => (
            <div key={`${title}-${row.itemSlot}-${row.itemId}-${index}`} className="guide-item-option">
              {index > 0 && linked ? <span className="guide-item-arrow">-&gt;</span> : null}
              {itemImageUrl(items, String(row.itemId)) ? (
                <img src={itemImageUrl(items, String(row.itemId))} alt={itemName(items, String(row.itemId))} title={itemName(items, String(row.itemId))} />
              ) : (
                <span className="item-pill">{row.itemId}</span>
              )}
              <div>
                <strong>{row.winRate.toFixed(2)}% WR</strong>
                <span>{formatNumber(row.games)} matches</span>
              </div>
            </div>
          ))}
        </div>
      ) : <EmptyState message={loading ? 'Loading items...' : 'No item sample yet.'} />}
      <small>{subtitle}</small>
    </PanelCard>
  );
}

function GuideStartingItemPanel({ title, subtitle, loadouts, fallbackRows, items, loading }: { title: string; subtitle: string; loadouts: GuideStartingLoadout[]; fallbackRows: GuideItemSlot[]; items?: ItemData; loading: boolean }) {
  return (
    <PanelCard className="guide-card guide-item-panel guide-starting-panel">
      <PanelTitle title={title} />
      {loadouts.length ? (
        <div className="guide-item-list">
          {loadouts.map((row) => {
            const itemIds = signatureItems(row.itemSignature);
            return (
              <div key={`starting-${normalizedItemSignature(row.itemSignature)}`} className="guide-item-option guide-starting-option">
                <div className="guide-starting-icons">
                  {itemIds.map((itemId, index) => (
                    itemImageUrl(items, String(itemId)) ? (
                      <img key={`${row.itemSignature}-${itemId}-${index}`} src={itemImageUrl(items, String(itemId))} alt={itemName(items, String(itemId))} title={itemName(items, String(itemId))} />
                    ) : (
                      <span key={`${row.itemSignature}-${itemId}-${index}`} className="item-pill">{itemId}</span>
                    )
                  ))}
                </div>
                <div>
                  <strong>{row.winRate.toFixed(2)}% WR</strong>
                  <span>{formatNumber(row.games)} matches</span>
                </div>
              </div>
            );
          })}
        </div>
      ) : fallbackRows.length ? (
        <div className="guide-item-list">
          {fallbackRows.map((row, index) => (
            <div key={`${title}-${row.itemSlot}-${row.itemId}-${index}`} className="guide-item-option">
              {itemImageUrl(items, String(row.itemId)) ? (
                <img src={itemImageUrl(items, String(row.itemId))} alt={itemName(items, String(row.itemId))} title={itemName(items, String(row.itemId))} />
              ) : (
                <span className="item-pill">{row.itemId}</span>
              )}
              <div>
                <strong>{row.winRate.toFixed(2)}% WR</strong>
                <span>{formatNumber(row.games)} matches</span>
              </div>
            </div>
          ))}
        </div>
      ) : <EmptyState message={loading ? 'Loading items...' : 'No starting item sample yet.'} />}
      <small>{subtitle}</small>
    </PanelCard>
  );
}

function formatNumber(value: number) {
  return new Intl.NumberFormat('en-US').format(value);
}

function signatureItems(signature: string) {
  return signature
    .split('-')
    .map((part) => Number(part))
    .filter((itemId) => itemId > 0);
}

function normalizedItemSignature(signature: string) {
  return signatureItems(signature).join('-');
}

function sortedSlotCandidates(rows: GuideItemSlot[], slot: number) {
  return rows
    .filter((row) => row.itemSlot === slot)
    .sort((a, b) => {
      const leftScore = guideItemSlotScore(a);
      const rightScore = guideItemSlotScore(b);
      if (leftScore !== rightScore) return rightScore - leftScore;
      if (a.confidence !== b.confidence) return b.confidence - a.confidence;
      if (a.games !== b.games) return b.games - a.games;
      return b.winRate - a.winRate;
    });
}

function uniqueItemRows(rows: GuideItemSlot[]) {
  const out: GuideItemSlot[] = [];
  const used = new Set<number>();
  for (const row of rows) {
    if (used.has(row.itemId)) continue;
    used.add(row.itemId);
    out.push(row);
  }
  return out;
}

function pickUniqueCoreRows(slotRows: GuideItemSlot[][]) {
  const used = new Set<number>();
  const out: GuideItemSlot[] = [];
  for (const candidates of slotRows) {
    const row = candidates.find((candidate) => !used.has(candidate.itemId));
    if (!row) continue;
    used.add(row.itemId);
    out.push(row);
  }
  return out;
}

function optionRows(rows: GuideItemSlot[], usedCoreItemIds: Set<number>) {
  const uniqueRows = uniqueItemRows(rows);
  const filtered = uniqueRows.filter((row) => !usedCoreItemIds.has(row.itemId));
  return (filtered.length ? filtered : uniqueRows).slice(0, 3);
}

function addDisplayedItems(used: Set<number>, rows: GuideItemSlot[]) {
  for (const row of rows) {
    used.add(row.itemId);
  }
}

function guideItemSlotScore(row: GuideItemSlot) {
  const reliability = Math.min(1, Math.sqrt(row.games / 200));
  return row.confidence * reliability;
}

function guideStartingLoadoutScore(row: GuideStartingLoadout) {
  const reliability = Math.min(1, Math.sqrt(row.games / 150));
  return row.confidence * reliability;
}
