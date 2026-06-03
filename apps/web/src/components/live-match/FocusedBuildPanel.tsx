import type { AnalyticsBuild, AnalyticsItemSlot, BuildAdviceResponse, BuildFilters, ChampionData, ChampionGuideItemPath, ItemData, LiveParticipant, RuneData, SummonerSpellData } from '../../api/types';
import { RoleIcon, roleLabel } from '../../lib/roles';
import { championByKey, championImageUrl } from '../../lib/staticData';
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
import { BuildParticipantPicker } from './BuildParticipantPicker';
import { BuildAdviceSetupStrip, BuildResultCard, type BuildPathDisplay } from './FocusedBuildResults';
import { selectBuildSlotRows } from './buildSlotSelection';
import { participantDisplayName } from './participantLabels';

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
  const matchupSummary = buildPathSummary(matchupItemSlots, items);
  const championSummary = buildPathSummary(championItemSlots, items);
  const matchupDelta = buildPathDelta(matchupSummary, championSummary);
  const activeParticipantKey = selectedParticipantKey || selection.participantKey;
  const activeOpponentKey = selectedOpponentKey || selection.opponentKey;
  const championName = champion?.name ?? String(selection.participant.championId);
  const opponentChampionName = opponentChampion?.name ?? String(selection.opponent.championId);
  const matchupHasExactSlots = matchupItemSlots.some((row) => !row.fallback);

  return (
    <section className={`focused-build-panel ${selection.side}`} aria-label="Focused build matchup">
      <div className="focused-build-command-row">
        <div className="focused-build-command-group build-target">
          <div className="focused-build-owner-card">
            {championUrl ? <img src={championUrl} alt={championName} /> : null}
            <span>
              <small>Builds For</small>
              <strong>{championName}</strong>
              <em>{playerName}</em>
              <b><RoleIcon role={selection.role} /> {roleLabel(selection.role)}</b>
            </span>
          </div>
          <BuildParticipantPicker
            title="Build Target"
            options={selection.participantOptions}
            selectedKey={activeParticipantKey}
            champions={champions}
            onSelect={onSelectParticipant}
          />
        </div>
        <div className="focused-build-command-group opponent-target">
          <div className="focused-build-context-card">
            <span className="focused-build-context-label">Matchup Context</span>
            <div>
              {opponentUrl ? <img src={opponentUrl} alt={opponentChampionName} /> : null}
              <span>
                <small>Against</small>
                <strong>{opponentChampionName}</strong>
                <em>{opponentName}</em>
              </span>
            </div>
            <p>Opponent filters the matchup sample. Build cards below remain for {championName}.</p>
          </div>
          <BuildParticipantPicker
            title="Opponent"
            options={selection.opponentOptions}
            selectedKey={activeOpponentKey}
            champions={champions}
            onSelect={onSelectOpponent}
          />
        </div>
      </div>
      <BuildAdviceSetupStrip buildAdvice={buildAdvice} spells={spells} runes={runes} loading={loading} />
      <div className="focused-build-results">
        <BuildResultCard
          title={matchupHasExactSlots ? `Best ${championName} build vs ${opponentChampionName}` : `${championName} baseline vs ${opponentChampionName}`}
          sample={matchupSample}
          summary={matchupSummary}
          comparison={matchupHasExactSlots ? matchupDelta : 'Baseline fallback'}
          notes={buildAdvice?.notes}
          side={selection.side}
          itemSlots={matchupItemSlots}
          buildPaths={buildPathDisplays(buildAdvice?.matchup.topBuilds)}
          loading={loading}
          items={items}
          emptyTitle="No matchup build yet"
          emptySubtitle={`Needs ${BUILD_MATCHUP_MIN_GAMES}+ stored games for this exact pairing`}
        />
        <BuildResultCard
          title={`Best overall ${championName} build`}
          sample={championSample}
          summary={championSummary}
          comparison="Champion-wide reference"
          notes={undefined}
          side={selection.side}
          itemSlots={championItemSlots}
          buildPaths={buildPathDisplays(buildAdvice?.champion.topBuilds, buildAdvice?.champion.topItemPaths)}
          loading={loading}
          items={items}
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

function highestSlotSample(itemSlots: AnalyticsItemSlot[]) {
  return itemSlots.reduce((max, row) => Math.max(max, row.games), 0);
}

function buildPathSummary(itemSlots: AnalyticsItemSlot[], items?: ItemData): BuildPathSummary | undefined {
  const shownRows = topItemSlotRows(itemSlots, items);
  const totalGames = shownRows.reduce((sum, row) => sum + row.games, 0);
  if (totalGames <= 0) return undefined;
  const weightedWinRate = shownRows.reduce((sum, row) => sum + (row.winRate * row.games), 0) / totalGames;
  return { weightedWinRate, totalGames };
}

function buildPathDisplays(builds?: AnalyticsBuild[], guidePaths?: ChampionGuideItemPath[]): BuildPathDisplay[] {
  const rows = builds
    ?.map((build, index) => ({
      key: `build-${index}-${build.core3Signature || build.finalItemsSignature}`,
      signature: build.core3Signature || build.finalItemsSignature,
      wins: build.wins,
      games: build.games,
      winRate: build.winRate,
      confidence: build.confidence,
    }))
    .filter((row) => row.signature && row.games > 0) ?? [];
  if (rows.length) return rows;
  return guidePaths
    ?.map((path, index) => ({
      key: `guide-${index}-${path.core3Signature || path.finalItemsSignature}`,
      signature: path.core3Signature || path.finalItemsSignature,
      wins: path.wins,
      games: path.games,
      winRate: path.winRate,
      confidence: path.confidence,
    }))
    .filter((row) => row.signature && row.games > 0) ?? [];
}

function buildPathDelta(matchup?: BuildPathSummary, baseline?: BuildPathSummary) {
  if (!matchup || !baseline) return '';
  const delta = matchup.weightedWinRate - baseline.weightedWinRate;
  const sign = delta > 0 ? '+' : '';
  return `${sign}${delta.toFixed(1)} pts vs baseline`;
}

function topItemSlotRows(itemSlots: AnalyticsItemSlot[], items?: ItemData) {
  return selectBuildSlotRows(itemSlots, items)
    .map((slot) => slot.row)
    .filter((row): row is AnalyticsItemSlot => Boolean(row));
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
