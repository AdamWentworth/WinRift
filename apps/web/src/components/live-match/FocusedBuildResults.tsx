import type { AnalyticsItemSlot, BuildAdviceResponse, ItemData, RuneData, SummonerSpellData } from '../../api/types';
import {
  itemImageUrl,
  itemName,
  parseRuneSignature,
  runeImageUrl,
  runeName,
  runeStyleImageUrl,
  runeStyleName,
  signatureItems,
  signatureSpells,
  summonerSpellImageUrl,
  summonerSpellName,
} from '../../lib/staticData';
import { StatShardGrid } from '../ui/StatShardDisplay';
import { StatusChip } from '../ui/StatusChip';
import type { BuildPathSummary, TeamSide } from './types';
import { selectBuildSlotRows } from './buildSlotSelection';

export type BuildResultSample = {
  label: string;
  tone: string;
};

export type BuildPathDisplay = {
  key: string;
  signature: string;
  wins: number;
  games: number;
  winRate: number;
  confidence?: number;
};

export function BuildResultCard({
  title,
  sample,
  summary,
  comparison,
  notes,
  side,
  itemSlots,
  buildPaths,
  loading,
  items,
  emptyTitle,
  emptySubtitle,
}: {
  title: string;
  sample: BuildResultSample;
  summary?: BuildPathSummary;
  comparison?: string;
  notes?: string[];
  side: TeamSide;
  itemSlots: AnalyticsItemSlot[];
  buildPaths: BuildPathDisplay[];
  loading: boolean;
  items?: ItemData;
  emptyTitle: string;
  emptySubtitle: string;
}) {
  const showPathRows = loading || buildPaths.length > 0;
  return (
    <article className="focused-build-result">
      <header>
        <span>
          <strong>{title}</strong>
          {summary ? (
            <small className="build-result-summary">
              {summary.weightedWinRate.toFixed(1)}% WR · {summary.totalGames} item samples
            </small>
          ) : null}
        </span>
        <div className="build-result-meta">
          {comparison ? <StatusChip as="b" className="build-delta" tone={comparison.includes('+') ? 'good' : comparison.includes('-') ? 'warn' : undefined}>{comparison}</StatusChip> : null}
          <StatusChip as="b" className="build-sample-chip" tone={sample.tone}>{sample.label}</StatusChip>
        </div>
      </header>
      {notes?.length ? (
        <div className="build-advice-notes">
          {notes.slice(0, 1).map((note) => <span key={note}>{compactBuildNote(note)}</span>)}
        </div>
      ) : null}
      <div className={`focused-build-result-body${showPathRows ? '' : ' slots-only'}`}>
        {showPathRows ? <BuildPathRows paths={buildPaths} items={items} loading={loading} /> : null}
        <div className="build-slot-signals">
          <div className="build-slot-heading">
            <span>Item slot reads</span>
          </div>
          <BuildSide
            side={side}
            itemSlots={itemSlots}
            loading={loading}
            items={items}
            emptyTitle={emptyTitle}
            emptySubtitle={emptySubtitle}
          />
        </div>
      </div>
    </article>
  );
}

export function BuildAdviceSetupStrip({
  buildAdvice,
  spells,
  runes,
  loading,
}: {
  buildAdvice?: BuildAdviceResponse;
  spells?: SummonerSpellData;
  runes?: RuneData;
  loading: boolean;
}) {
  const runeRow = buildAdvice?.champion.topRunes[0];
  const spellRow = buildAdvice?.champion.topSpells[0];
  const parsedRunes = parseRuneSignature(runeRow?.runeSignature ?? '');
  const spellIds = signatureSpells(spellRow?.spellSignature ?? '');
  const primaryRuneStyleSrc = runeStyleImageUrl(runes, parsedRunes.primaryStyleId);
  if (loading) {
    return <div className="build-setup-strip muted">Loading runes and spells...</div>;
  }
  if (!runeRow && !spellRow) {
    return null;
  }
  return (
    <div className="build-setup-strip">
      <div className="build-setup-block">
        <span>Top Rune Setup</span>
        <div className="build-setup-icons">
          {primaryRuneStyleSrc ? (
            <img src={primaryRuneStyleSrc} alt={runeStyleName(runes, parsedRunes.primaryStyleId)} title={runeStyleName(runes, parsedRunes.primaryStyleId)} />
          ) : null}
          {parsedRunes.runeIds.slice(0, 4).map((runeId) => {
            const src = runeImageUrl(runes, runeId);
            return src ? <img key={runeId} src={src} alt={runeName(runes, runeId)} title={runeName(runes, runeId)} /> : null;
          })}
        </div>
        <StatShardGrid selectedIds={parsedRunes.statPerks} className="build-setup-shards" />
        {runeRow ? <em>{runeRow.winRate.toFixed(1)}% WR · {runeRow.games} games</em> : null}
      </div>
      <div className="build-setup-block">
        <span>Top Spells</span>
        <div className="build-setup-icons">
          {spellIds.map((spellId) => {
            const src = summonerSpellImageUrl(spells, spellId);
            return src ? <img key={spellId} src={src} alt={summonerSpellName(spells, spellId)} title={summonerSpellName(spells, spellId)} /> : null;
          })}
        </div>
        {spellRow ? <em>{spellRow.winRate.toFixed(1)}% WR · {spellRow.games} games</em> : null}
      </div>
    </div>
  );
}

function BuildPathRows({ paths, items, loading }: { paths: BuildPathDisplay[]; items?: ItemData; loading: boolean }) {
  if (loading) {
    return <div className="build-path-signals muted">Loading actual build paths...</div>;
  }
  if (!paths.length) {
    return (
      <div className="build-path-signals muted">
        <strong>No stable full path yet</strong>
        <span>Slot signals below are still useful, but this exact path sample is thin.</span>
      </div>
    );
  }
  return (
    <div className="build-path-signals" aria-label="Actual build paths from stored games">
      <span className="build-path-signals-title">Actual build paths</span>
      {paths.slice(0, 3).map((path, index) => (
        <div className="build-path-signal-row" key={path.key}>
          <b>{index + 1}</b>
          <BuildPathItems signature={path.signature} items={items} />
          <span className="build-path-stat">
            <strong>{path.winRate.toFixed(1)}%</strong>
            <em>{path.games} games</em>
          </span>
        </div>
      ))}
    </div>
  );
}

function BuildPathItems({ signature, items }: { signature: string; items?: ItemData }) {
  const ids = signatureItems(signature).slice(0, 6);
  if (!ids.length) {
    return <span className="build-path-items empty">No item path</span>;
  }
  return (
    <span className="build-path-items">
      {ids.map((itemId) => {
        const imageUrl = itemImageUrl(items, itemId);
        const name = itemName(items, itemId);
        return imageUrl ? <img key={itemId} src={imageUrl} alt={name} title={name} /> : <em key={itemId}>{itemId}</em>;
      })}
    </span>
  );
}

function BuildSide({
  side,
  itemSlots,
  loading,
  items,
  emptyTitle = 'No matchup build yet',
  emptySubtitle = 'Needs more stored games',
}: {
  side: 'blue' | 'red';
  itemSlots: AnalyticsItemSlot[];
  loading: boolean;
  items?: ItemData;
  emptyTitle?: string;
  emptySubtitle?: string;
}) {
  if (loading) {
    return <div className={`build-side ${side} muted`}>Loading item patterns...</div>;
  }
  if (!itemSlots.length) {
    return (
      <div className={`build-side ${side} muted build-empty-state`}>
        <strong>{emptyTitle}</strong>
        <span>{emptySubtitle}</span>
      </div>
    );
  }

  const bestBySlot = selectBuildSlotRows(itemSlots, items);

  return (
    <div className={`build-side ${side}`}>
      {bestBySlot.map(({ slot, row, commonRow }) => (
        row ? (
          <ItemSlotLine
            key={`${row.itemSlot}-${row.itemId}`}
            row={row}
            commonRow={commonRow && commonRow.itemId !== row.itemId ? commonRow : undefined}
            items={items}
          />
        ) : <MissingItemSlotLine key={slot} slot={slot} />
      ))}
    </div>
  );
}

function ItemSlotLine({ row, commonRow, items }: { row: AnalyticsItemSlot; commonRow?: AnalyticsItemSlot; items?: ItemData }) {
  const itemId = String(row.itemId);
  const imageUrl = itemImageUrl(items, itemId);
  const name = itemName(items, itemId);
  const commonName = commonRow ? itemName(items, String(commonRow.itemId)) : '';
  const commonTitle = commonRow ? ` Most played in sample: ${commonName}, ${commonRow.winRate.toFixed(1)}% over ${commonRow.games} games.` : '';
  return (
    <div className={`item-slot-column${row.games < 5 ? ' low-sample-item' : ''}`} title={`${ordinal(row.itemSlot)} item: ${name}. ${row.winRate.toFixed(1)}% over ${row.games} games.${commonTitle}`}>
      <span className="item-slot-number">{ordinal(row.itemSlot)}</span>
      {imageUrl ? <img src={imageUrl} alt={name} title={name} /> : <span className="item-pill">{row.itemId}</span>}
      <div className="item-slot-stats">
        <strong>{row.winRate.toFixed(1)}%</strong>
        <span>{row.games}g{row.games < 5 ? ' · thin' : ''}</span>
      </div>
      {commonRow ? <small className="item-common-note">Common {commonRow.games}g</small> : null}
    </div>
  );
}

function MissingItemSlotLine({ slot }: { slot: number }) {
  return (
    <div className="item-slot-column missing-item-slot">
      <span className="item-slot-number">{ordinal(slot)}</span>
      <span className="item-slot-empty">--</span>
      <div className="item-slot-stats">
        <strong>--</strong>
      </div>
    </div>
  );
}

function compactBuildNote(note: string) {
  if (note.includes('other stored patches')) return 'Some slots use exact-matchup data from older stored patches.';
  if (note.includes('No current-patch matchup')) return 'No current-patch slot sample yet; using exact matchup history.';
  if (note.includes('No exact matchup item slots')) return 'Exact matchup sample is too thin; baseline shown instead.';
  if (note.includes('champion-wide fallback')) return 'Some slots fall back to champion-wide data.';
  return note;
}

function ordinal(value: number) {
  if (value === 0) return 'Start';
  if (value === 1) return '1st';
  if (value === 2) return '2nd';
  if (value === 3) return '3rd';
  return `${value}th`;
}
