import type { AnalyticsItemSlot, BuildAdviceResponse, BuildFilters, ChampionData, ItemData, LiveParticipant, RuneData, SummonerSpellData } from '../../api/types';
import { RoleIcon, roleLabel } from '../../lib/roles';
import {
  championByKey,
  championImageUrl,
  itemImageUrl,
  itemName,
  parseRuneSignature,
  runeImageUrl,
  runeName,
  runeStyleImageUrl,
  runeStyleName,
  signatureSpells,
  summonerSpellImageUrl,
  summonerSpellName,
} from '../../lib/staticData';
import {
  BUILD_BASELINE_MIN_GAMES,
  BUILD_MATCHUP_MIN_GAMES,
  BUILD_SLOT_CANDIDATE_LIMIT,
  roles,
  type BuildParticipantOption,
  type BuildPathSummary,
  type FocusedBuildSelection,
  type TeamSide,
} from './types';
import { hasSmite, participantKey, sameParticipantIdentity } from './utils';
import { StatusChip } from '../ui/StatusChip';

export function FocusedBuildPanel({
  selection,
  champions,
  items,
  spells,
  runes,
  buildAdvice,
  loading,
  selectedParticipantKey,
  onSelectParticipant,
  selectedOpponentKey,
  onSelectOpponent,
}: {
  selection?: FocusedBuildSelection;
  champions?: ChampionData;
  items?: ItemData;
  spells?: SummonerSpellData;
  runes?: RuneData;
  buildAdvice?: BuildAdviceResponse;
  loading: boolean;
  selectedParticipantKey: string;
  onSelectParticipant: (key: string) => void;
  selectedOpponentKey: string;
  onSelectOpponent: (key: string) => void;
}) {
  if (!selection) {
    return <section className="focused-build-panel build-empty-state">No live player found for build focus.</section>;
  }
  const champion = championByKey(champions, selection.participant.championId);
  const opponentChampion = championByKey(champions, selection.opponent.championId);
  const championUrl = championImageUrl(champions, selection.participant.championId);
  const opponentUrl = championImageUrl(champions, selection.opponent.championId);
  const playerName = participantDisplayName(selection.participant);
  const opponentName = participantDisplayName(selection.opponent);
  const matchupItemSlots = buildAdvice?.matchup.itemSlots ?? [];
  const championItemSlots = buildAdvice?.champion.itemSlots ?? [];
  const matchupSample = buildAdviceSample(buildAdvice?.matchup.sample, matchupItemSlots);
  const championSample = buildAdviceSample(buildAdvice?.champion.sample, championItemSlots);
  const matchupScopeLabel = buildScopeLabel(matchupItemSlots, buildAdvice?.matchup.sample.scopeLabels);
  const championScopeLabel = buildScopeLabel(championItemSlots, buildAdvice?.champion.sample.scopeLabels);
  const matchupSummary = buildPathSummary(matchupItemSlots);
  const championSummary = buildPathSummary(championItemSlots);
  const matchupDelta = buildPathDelta(matchupSummary, championSummary);
  const activeParticipantKey = selectedParticipantKey || selection.participantKey;
  const activeOpponentKey = selectedOpponentKey || selection.opponentKey;

  return (
    <section className={`focused-build-panel ${selection.side}`} aria-label="Focused build matchup">
      <div className="focused-build-header">
        <div className="focused-build-player">
          {championUrl ? <img src={championUrl} alt={champion?.name ?? 'Champion'} /> : null}
          <span>
            <small><RoleIcon role={selection.role} /> {roleLabel(selection.role)}</small>
            <strong>{champion?.name ?? selection.participant.championId}</strong>
            <em>{playerName}</em>
          </span>
        </div>
        <div className="focused-build-versus">
          <b>vs</b>
        </div>
        <div className="focused-build-player enemy">
          {opponentUrl ? <img src={opponentUrl} alt={opponentChampion?.name ?? 'Opponent'} /> : null}
          <span>
            <small>Opponent</small>
            <strong>{opponentChampion?.name ?? selection.opponent.championId}</strong>
            <em>{opponentName}</em>
          </span>
        </div>
      </div>
      <div className="focused-build-controls">
        <BuildParticipantPicker
          title="Build For"
          options={selection.participantOptions}
          selectedKey={activeParticipantKey}
          champions={champions}
          onSelect={onSelectParticipant}
        />
        <BuildParticipantPicker
          title="Against"
          options={selection.opponentOptions}
          selectedKey={activeOpponentKey}
          champions={champions}
          onSelect={onSelectOpponent}
        />
      </div>
      <BuildAdviceSetupStrip buildAdvice={buildAdvice} spells={spells} runes={runes} loading={loading} />
      <div className="focused-build-results">
        <BuildResultCard
          title="Matchup Build"
          description={`${champion?.name ?? selection.participant.championId} vs ${opponentChampion?.name ?? selection.opponent.championId}`}
          sample={matchupSample}
          summary={matchupSummary}
          comparison={matchupDelta}
          scopeLabel={matchupScopeLabel}
          notes={buildAdvice?.notes}
          side={selection.side}
          itemSlots={matchupItemSlots}
          loading={loading}
          items={items}
          minGames={BUILD_MATCHUP_MIN_GAMES}
          emptyTitle="No matchup build yet"
          emptySubtitle={`Needs ${BUILD_MATCHUP_MIN_GAMES}+ stored games for this exact pairing`}
        />
        <BuildResultCard
          title="Champion Baseline"
          description={`Highest winrate ${champion?.name ?? selection.participant.championId} items overall`}
          sample={championSample}
          summary={championSummary}
          comparison="Champion-wide reference"
          scopeLabel={championScopeLabel}
          notes={undefined}
          side={selection.side}
          itemSlots={championItemSlots}
          loading={loading}
          items={items}
          minGames={BUILD_BASELINE_MIN_GAMES}
          emptyTitle="No champion build yet"
          emptySubtitle={`Needs ${BUILD_BASELINE_MIN_GAMES}+ stored games for this champion`}
        />
      </div>
    </section>
  );
}

export function buildAdviceFilters(participant: LiveParticipant, opponent: LiveParticipant, role: string, patch?: string): BuildFilters & { championMinGames: number } {
  return {
    championId: participant.championId,
    role,
    opponentChampionId: opponent.championId,
    itemContext: itemContextForParticipant(participant, role),
    patch,
    minGames: BUILD_MATCHUP_MIN_GAMES,
    championMinGames: BUILD_BASELINE_MIN_GAMES,
    limit: BUILD_SLOT_CANDIDATE_LIMIT,
  };
}

export function focusedBuildSelection(
  searchedParticipant: LiveParticipant | undefined,
  blueTeam: LiveParticipant[],
  redTeam: LiveParticipant[],
  selectedParticipantKey: string,
  selectedOpponentKey: string,
): FocusedBuildSelection | undefined {
  const participantOptions = buildParticipantOptions(blueTeam, redTeam);
  if (!participantOptions.length) return undefined;
  const searchedOption = searchedParticipant
    ? participantOptions.find((option) => sameParticipantIdentity(option.participant, searchedParticipant))
    : undefined;
  const selectedOption = participantOptions.find((option) => option.key === selectedParticipantKey);
  const target = selectedOption ?? searchedOption;
  if (!target) return undefined;

  const opponentOptions = participantOptions.filter((option) => option.side !== target.side);
  if (!opponentOptions.length) return undefined;
  const selectedOpponent = opponentOptions.find((option) => option.key === selectedOpponentKey);
  const laneOpponent = opponentOptions.find((option) => option.index === target.index);
  const opponent = selectedOpponent ?? laneOpponent ?? opponentOptions[0];

  return {
    side: target.side,
    role: target.role,
    participantKey: target.key,
    participant: target.participant,
    opponentKey: opponent.key,
    opponent: opponent.participant,
    participantOptions,
    opponentOptions,
  };
}

function BuildParticipantPicker({
  title,
  options,
  selectedKey,
  champions,
  onSelect,
}: {
  title: string;
  options: BuildParticipantOption[];
  selectedKey: string;
  champions?: ChampionData;
  onSelect: (key: string) => void;
}) {
  return (
    <div className="focused-build-picker">
      <span>{title}</span>
      <div>
        {options.map((option) => {
          const optionChampion = championByKey(champions, option.participant.championId);
          const optionUrl = championImageUrl(champions, option.participant.championId);
          const labelName = participantDisplayName(option.participant);
          return (
            <button
              className={`${option.side}${option.key === selectedKey ? ' selected' : ''}`}
              key={option.key}
              onClick={() => onSelect(option.key)}
              type="button"
              aria-label={`${title} ${labelName}`}
            >
              {optionUrl ? <img src={optionUrl} alt="" /> : null}
              <strong>{optionChampion?.name ?? option.participant.championId}</strong>
              <em>{labelName}</em>
              <small><RoleIcon role={option.role} /> {roleLabel(option.role)}</small>
            </button>
          );
        })}
      </div>
    </div>
  );
}

function BuildResultCard({
  title,
  description,
  sample,
  summary,
  comparison,
  scopeLabel,
  notes,
  side,
  itemSlots,
  loading,
  items,
  minGames,
  emptyTitle,
  emptySubtitle,
}: {
  title: string;
  description: string;
  sample: { label: string; tone: string };
  summary?: BuildPathSummary;
  comparison?: string;
  scopeLabel: string;
  notes?: string[];
  side: TeamSide;
  itemSlots: AnalyticsItemSlot[];
  loading: boolean;
  items?: ItemData;
  minGames: number;
  emptyTitle: string;
  emptySubtitle: string;
}) {
  return (
    <article className="focused-build-result">
      <header>
        <span>
          <strong>{title}</strong>
          <em>{description}</em>
          {summary ? (
            <small className="build-result-summary">
              {summary.weightedWinRate.toFixed(1)}% shown-item WR · {summary.totalGames} shown samples
            </small>
          ) : null}
        </span>
        <div>
          {comparison ? <StatusChip as="b" className="build-delta" tone={comparison.includes('+') ? 'good' : comparison.includes('-') ? 'warn' : undefined}>{comparison}</StatusChip> : null}
          <StatusChip as="b" className="build-sample-chip" tone={sample.tone}>{sample.label}</StatusChip>
          <StatusChip as="small" className="build-min-sample">{minGames}+ games/item</StatusChip>
          {scopeLabel ? <StatusChip as="small" className="build-scope-label">{scopeLabel}</StatusChip> : null}
        </div>
      </header>
      {notes?.length ? (
        <div className="build-advice-notes">
          {notes.slice(0, 2).map((note) => <span key={note}>{note}</span>)}
        </div>
      ) : null}
      <BuildSide
        side={side}
        itemSlots={itemSlots}
        loading={loading}
        items={items}
        emptyTitle={emptyTitle}
        emptySubtitle={emptySubtitle}
      />
    </article>
  );
}

function BuildAdviceSetupStrip({
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

  const bestBySlot = [1, 2, 3, 4, 5, 6].map((slot) => {
    const slotRows = itemSlots.filter((candidate) => candidate.itemSlot === slot);
    return {
      slot,
      row: slotRows[0],
      commonRow: mostPlayedItemSlot(slotRows),
    };
  });

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
      {commonRow ? <small className="item-common-note">Most common: {commonRow.games}g</small> : null}
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
        <span>No sample</span>
      </div>
    </div>
  );
}

function buildParticipantOptions(blueTeam: LiveParticipant[], redTeam: LiveParticipant[]): BuildParticipantOption[] {
  return [
    ...teamBuildParticipantOptions('blue', blueTeam),
    ...teamBuildParticipantOptions('red', redTeam),
  ];
}

function teamBuildParticipantOptions(side: TeamSide, team: LiveParticipant[]): BuildParticipantOption[] {
  return team.map((participant, index) => ({
    key: buildParticipantOptionKey(side, participant, index),
    side,
    role: roles[index] ?? 'UNKNOWN',
    index,
    participant,
  }));
}

function buildParticipantOptionKey(side: TeamSide, participant: LiveParticipant, index: number) {
  return `${side}-${participantKey(participant, index)}`;
}

function itemContextForParticipant(participant: LiveParticipant, role: string): BuildFilters['itemContext'] {
  if (hasSmite(participant)) return 'JUNGLE';
  if (role === 'UTILITY') return 'SUPPORT';
  return undefined;
}

function participantDisplayName(participant: LiveParticipant) {
  return participant.riotId || participant.summonerName || 'Unknown player';
}

function highestSlotSample(itemSlots: AnalyticsItemSlot[]) {
  return itemSlots.reduce((max, row) => Math.max(max, row.games), 0);
}

function buildPathSummary(itemSlots: AnalyticsItemSlot[]): BuildPathSummary | undefined {
  const shownRows = topItemSlotRows(itemSlots);
  const totalGames = shownRows.reduce((sum, row) => sum + row.games, 0);
  if (totalGames <= 0) return undefined;
  const weightedWinRate = shownRows.reduce((sum, row) => sum + (row.winRate * row.games), 0) / totalGames;
  return { weightedWinRate, totalGames };
}

function buildPathDelta(matchup?: BuildPathSummary, baseline?: BuildPathSummary) {
  if (!matchup || !baseline) return '';
  const delta = matchup.weightedWinRate - baseline.weightedWinRate;
  const sign = delta > 0 ? '+' : '';
  return `${sign}${delta.toFixed(1)} pts vs baseline`;
}

function topItemSlotRows(itemSlots: AnalyticsItemSlot[]) {
  return [1, 2, 3, 4, 5, 6]
    .map((slot) => itemSlots.find((candidate) => candidate.itemSlot === slot))
    .filter((row): row is AnalyticsItemSlot => Boolean(row));
}

function mostPlayedItemSlot(rows: AnalyticsItemSlot[]) {
  return rows.reduce<AnalyticsItemSlot | undefined>((best, row) => {
    if (!best) return row;
    if (row.games !== best.games) return row.games > best.games ? row : best;
    if (row.winRate !== best.winRate) return row.winRate > best.winRate ? row : best;
    return row.itemId < best.itemId ? row : best;
  }, undefined);
}

function buildAdviceSample(sample: BuildAdviceResponse['matchup']['sample'] | undefined, itemSlots: AnalyticsItemSlot[]) {
  if (sample) {
    const samples = sample.maxGames;
    const tone = sample.sampleQuality === 'strong' ? 'strong' : sample.sampleQuality === 'moderate' ? 'useful' : sample.sampleQuality === 'early' ? 'early' : sample.sampleQuality === 'tiny' ? 'thin' : 'none';
    return {
      label: samples > 0 ? `${samples} samples · ${sample.sampleQualityLabel.replace(/ sample$/i, '').toLowerCase()}` : sample.sampleQualityLabel,
      tone,
    };
  }
  return buildSampleQuality(highestSlotSample(itemSlots));
}

function buildSampleQuality(samples: number) {
  if (samples <= 0) return { label: 'No samples', tone: 'none' };
  if (samples < 5) return { label: `${samples} sample${samples === 1 ? '' : 's'} · thin`, tone: 'thin' };
  if (samples < 15) return { label: `${samples} samples · early`, tone: 'early' };
  if (samples < 50) return { label: `${samples} samples · useful`, tone: 'useful' };
  return { label: `${samples} samples · strong`, tone: 'strong' };
}

function buildScopeLabel(itemSlots: AnalyticsItemSlot[], scopeLabels?: string[]) {
  const labels = scopeLabels?.length ? scopeLabels : [...new Set(itemSlots.map((row) => row.sampleScopeLabel).filter(Boolean))];
  if (!labels.length) return '';
  if (labels.length === 1) return labels[0] ?? '';
  return 'Mixed fallback samples';
}

function ordinal(value: number) {
  if (value === 1) return '1st';
  if (value === 2) return '2nd';
  if (value === 3) return '3rd';
  return `${value}th`;
}
