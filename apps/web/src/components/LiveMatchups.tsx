import { useQuery } from '@tanstack/react-query';
import { GripVertical, Network, Swords, Users } from 'lucide-react';
import { useEffect, useMemo, useState, type DragEvent } from 'react';
import type { AnalyticsItemSlot, BuildFilters, ChampionData, ChampionRecord, ChampionRoleRate, ItemData, LiveGame, LiveParticipant, RankedRecord, RuneData, SummonerSpellData, WinConditionAnalysisResponse, WinConditionMetric, WinConditionTeamProfile } from '../api/types';
import { getChampionRoleRates, getItemSlotsBatch, getWinConditionAnalysis } from '../api/client';
import {
  championByKey,
  championImageUrl,
  championSplashUrl,
  itemImageUrl,
  itemName,
  rankIconUrl,
  rankLabel,
  runeImageUrl,
  runeName,
  runeStyleName,
  summonerSpellImageUrl,
  summonerSpellName,
} from '../lib/staticData';

const roles = ['TOP', 'JUNGLE', 'MIDDLE', 'BOTTOM', 'UTILITY'];
const roleLabels: Record<string, string> = {
  TOP: 'Top',
  JUNGLE: 'Jungle',
  MIDDLE: 'Mid',
  BOTTOM: 'Bot',
  UTILITY: 'Support',
};
const BUILD_MATCHUP_MIN_GAMES = 5;
const BUILD_BASELINE_MIN_GAMES = 10;

type Props = {
  liveGame: LiveGame;
  champions?: ChampionData;
  items?: ItemData;
  spells?: SummonerSpellData;
  runes?: RuneData;
};

type TeamSide = 'blue' | 'red';
type LiveMode = 'match' | 'builds' | 'winConditions';

type DraggedCard = {
  side: TeamSide;
  index: number;
};

type RoleRateMap = Map<number, Map<string, ChampionRoleRate>>;

type BuildParticipantOption = {
  key: string;
  side: TeamSide;
  role: string;
  index: number;
  participant: LiveParticipant;
};

type FocusedBuildSelection = {
  side: TeamSide;
  role: string;
  participantKey: string;
  participant: LiveParticipant;
  opponentKey: string;
  opponent: LiveParticipant;
  participantOptions: BuildParticipantOption[];
  opponentOptions: BuildParticipantOption[];
};

export function LiveMatchups({ liveGame, champions, items, spells, runes }: Props) {
  const [now, setNow] = useState(() => Date.now());
  const [liveMode, setLiveMode] = useState<LiveMode>('match');
  const liveChampionIds = useMemo(() => uniqueChampionIds(liveGame.participants), [liveGame.participants]);
  const patchBucket = useMemo(() => patchBucketFromVersion(champions?.version), [champions?.version]);
  const showBuildMode = liveMode === 'builds';
  const showWinConditionMode = liveMode === 'winConditions';
  const roleRatesQuery = useQuery({
    queryKey: ['champion-role-rates', liveGame.gameQueueConfigId, liveChampionIds],
    queryFn: () => getChampionRoleRates(liveChampionIds, liveGame.gameQueueConfigId),
    enabled: liveChampionIds.length > 0,
    staleTime: 5 * 60_000,
  });
  const roleRates = useMemo(() => buildRoleRateMap(roleRatesQuery.data?.results), [roleRatesQuery.data?.results]);
  const initialBlue = useMemo(() => orderTeam(liveGame.participants.filter((participant) => participant.teamId === 100), roleRates), [liveGame.participants, roleRates]);
  const initialRed = useMemo(() => orderTeam(liveGame.participants.filter((participant) => participant.teamId === 200), roleRates), [liveGame.participants, roleRates]);
  const [blueTeam, setBlueTeam] = useState(initialBlue);
  const [redTeam, setRedTeam] = useState(initialRed);
  const [draggedCard, setDraggedCard] = useState<DraggedCard | null>(null);
  const [dragTarget, setDragTarget] = useState<DraggedCard | null>(null);
  const [manualOrder, setManualOrder] = useState(false);
  const [selectedLaneIndex, setSelectedLaneIndex] = useState(0);
  const [selectedBuildParticipantKey, setSelectedBuildParticipantKey] = useState('');
  const [selectedBuildOpponentKey, setSelectedBuildOpponentKey] = useState('');
  const blueChampionIds = useMemo(() => teamChampionIds(blueTeam), [blueTeam]);
  const redChampionIds = useMemo(() => teamChampionIds(redTeam), [redTeam]);
  const yourSide = livePlayerSide(liveGame);
  const searchedParticipant = liveGame.participants.find((candidate) => idsMatch(candidate.puuid, liveGame.puuid));
  const focusedBuild = useMemo(() => {
    return focusedBuildSelection(searchedParticipant, blueTeam, redTeam, selectedBuildParticipantKey, selectedBuildOpponentKey);
  }, [blueTeam, redTeam, searchedParticipant, selectedBuildOpponentKey, selectedBuildParticipantKey]);
  const focusedBuildFilters = useMemo(() => (
    focusedBuild ? buildFilters(focusedBuild.participant, focusedBuild.opponent, focusedBuild.role, patchBucket) : undefined
  ), [focusedBuild, patchBucket]);
  const championBuildFilters = useMemo(() => (
    focusedBuild ? championBuildFiltersFor(focusedBuild.participant, focusedBuild.role, patchBucket) : undefined
  ), [focusedBuild, patchBucket]);
  const winConditionQuery = useQuery({
    queryKey: ['live-win-conditions', liveGame.gameQueueConfigId, patchBucket, blueChampionIds, redChampionIds],
    queryFn: () => getWinConditionAnalysis({
      blueChampionIds,
      redChampionIds,
      queueId: liveGame.gameQueueConfigId,
      patch: patchBucket,
      minGames: 5,
    }),
    enabled: showWinConditionMode && blueChampionIds.length === 5 && redChampionIds.length === 5,
    staleTime: 60_000,
  });

  useEffect(() => {
    setManualOrder(false);
    setDraggedCard(null);
    setDragTarget(null);
    setSelectedLaneIndex(0);
    setSelectedBuildParticipantKey('');
    setSelectedBuildOpponentKey('');
  }, [liveGame.gameId]);

  useEffect(() => {
    if (manualOrder) return;
    setBlueTeam(initialBlue);
    setRedTeam(initialRed);
    setDraggedCard(null);
    setDragTarget(null);
  }, [initialBlue, initialRed, manualOrder]);

  useEffect(() => {
    if (!liveGame.gameStartTime) return undefined;
    const interval = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(interval);
  }, [liveGame.gameStartTime]);

  const moveCardToIndex = (side: TeamSide, fromIndex: number, toIndex: number) => {
    setManualOrder(true);
    if (side === 'blue') {
      setBlueTeam((team) => moveParticipantToIndex(team, fromIndex, toIndex));
      return;
    }
    setRedTeam((team) => moveParticipantToIndex(team, fromIndex, toIndex));
  };

  const focusedItemSlotQuery = useQuery({
    queryKey: ['live-focused-item-slots', focusedBuildFilters, championBuildFilters],
    queryFn: () => getItemSlotsBatch([
      { key: 'matchup', ...focusedBuildFilters! },
      { key: 'champion', ...championBuildFilters! },
    ]),
    enabled: showBuildMode && Boolean(focusedBuildFilters && championBuildFilters),
    staleTime: 30_000,
  });

  const matchupItemSlots = focusedItemSlotQuery.data?.results.find((result) => result.key === 'matchup')?.results ?? [];
  const championItemSlots = focusedItemSlotQuery.data?.results.find((result) => result.key === 'champion')?.results ?? [];

  return (
    <div className="game-board">
      <MatchHeader
        liveGame={liveGame}
        now={now}
        patch={patchBucket}
        searchedParticipant={searchedParticipant}
        yourSide={yourSide}
        blueTeam={blueTeam}
        redTeam={redTeam}
      />
      <div className="live-mode-layout">
        <LiveModeRail mode={liveMode} onChange={setLiveMode} />
        <div className="live-mode-content">
          {manualOrder && liveMode !== 'builds' ? (
            <div className="board-actions">
              <button
                type="button"
                onClick={() => {
                  setManualOrder(false);
                  setBlueTeam(initialBlue);
                  setRedTeam(initialRed);
                  setDraggedCard(null);
                  setDragTarget(null);
                }}
              >
                Reset Lane Order
              </button>
            </div>
          ) : null}
          {showBuildMode ? (
            <FocusedBuildPanel
              selection={focusedBuild}
              champions={champions}
              items={items}
              matchupItemSlots={matchupItemSlots}
              championItemSlots={championItemSlots}
              loading={focusedItemSlotQuery.isLoading}
              selectedParticipantKey={selectedBuildParticipantKey}
              onSelectParticipant={(key) => {
                setSelectedBuildParticipantKey(key);
                setSelectedBuildOpponentKey('');
              }}
              selectedOpponentKey={selectedBuildOpponentKey}
              onSelectOpponent={setSelectedBuildOpponentKey}
            />
          ) : (
            <div className="cards-container">
              <LaneTabs selectedIndex={selectedLaneIndex} onSelect={setSelectedLaneIndex} />
              <LaneHeader />
              <div className="champion-row blue-row">
                {blueTeam.map((participant, index) => (
                  <LiveChampionCard
                    key={participantKey(participant, index)}
                    participant={participant}
                    index={index}
                    role={roles[index]}
                    side="blue"
                    champions={champions}
                    spells={spells}
                    runes={runes}
                    dragging={draggedCard?.side === 'blue' && draggedCard.index === index}
                    dropTarget={dragTarget?.side === 'blue' && dragTarget.index === index}
                    onDragStart={() => setDraggedCard({ side: 'blue', index })}
                    mobileActive={index === selectedLaneIndex}
                    onDragOver={(event) => {
                      event.preventDefault();
                      setDragTarget({ side: 'blue', index });
                    }}
                    onDrop={(event) => {
                      event.preventDefault();
                      if (draggedCard?.side === 'blue') {
                        moveCardToIndex('blue', draggedCard.index, index);
                      }
                      setDraggedCard(null);
                      setDragTarget(null);
                    }}
                    onDragEnd={() => {
                      setDraggedCard(null);
                      setDragTarget(null);
                    }}
                  />
                ))}
              </div>
              {showWinConditionMode && blueChampionIds.length === 5 && redChampionIds.length === 5 ? (
                <WinConditionPanel
                  analysis={winConditionQuery.data}
                  yourSide={yourSide}
                  loading={winConditionQuery.isLoading}
                  error={winConditionQuery.error instanceof Error ? winConditionQuery.error.message : undefined}
                />
              ) : null}
              <div className="champion-row red-row">
                {redTeam.map((participant, index) => (
                  <LiveChampionCard
                    key={participantKey(participant, index)}
                    participant={participant}
                    index={index}
                    role={roles[index]}
                    side="red"
                    champions={champions}
                    spells={spells}
                    runes={runes}
                    dragging={draggedCard?.side === 'red' && draggedCard.index === index}
                    dropTarget={dragTarget?.side === 'red' && dragTarget.index === index}
                    onDragStart={() => setDraggedCard({ side: 'red', index })}
                    mobileActive={index === selectedLaneIndex}
                    onDragOver={(event) => {
                      event.preventDefault();
                      setDragTarget({ side: 'red', index });
                    }}
                    onDrop={(event) => {
                      event.preventDefault();
                      if (draggedCard?.side === 'red') {
                        moveCardToIndex('red', draggedCard.index, index);
                      }
                      setDraggedCard(null);
                      setDragTarget(null);
                    }}
                    onDragEnd={() => {
                      setDraggedCard(null);
                      setDragTarget(null);
                    }}
                  />
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

const liveModeOptions: Array<{
  id: LiveMode;
  label: string;
  kicker: string;
  description: string;
  icon: typeof Swords;
}> = [
  {
    id: 'match',
    label: 'Match',
    kicker: 'Scout',
    description: 'Player cards and live match context',
    icon: Users,
  },
  {
    id: 'builds',
    label: 'Builds',
    kicker: 'Focused',
    description: 'Focused item path matchup stats',
    icon: Swords,
  },
  {
    id: 'winConditions',
    label: 'Win Conditions',
    kicker: 'Strategy',
    description: 'Team identity and timing',
    icon: Network,
  },
];

function LiveModeRail({ mode, onChange }: { mode: LiveMode; onChange: (mode: LiveMode) => void }) {
  return (
    <nav className="live-mode-rail" aria-label="Live analytics mode">
      {liveModeOptions.map((option) => {
        const Icon = option.icon;
        const selected = option.id === mode;
        return (
          <button
            aria-label={`Show ${option.label} mode`}
            aria-pressed={selected}
            className={`live-mode-button${selected ? ' selected' : ''}`}
            key={option.id}
            onClick={() => onChange(option.id)}
            title={option.description}
            type="button"
          >
            <Icon aria-hidden="true" size={20} strokeWidth={2.4} />
            <span>
              <strong>{option.label}</strong>
              <em>{option.kicker}</em>
            </span>
          </button>
        );
      })}
    </nav>
  );
}

function MatchHeader({
  liveGame,
  now,
  patch,
  searchedParticipant,
  yourSide,
  blueTeam,
  redTeam,
}: {
  liveGame: LiveGame;
  now: number;
  patch?: string;
  searchedParticipant?: LiveParticipant;
  yourSide: TeamSide;
  blueTeam: LiveParticipant[];
  redTeam: LiveParticipant[];
}) {
  const blueAverage = averageRankLabel(blueTeam);
  const redAverage = averageRankLabel(redTeam);
  const searchedName = searchedParticipant?.riotId || searchedParticipant?.summonerName || 'Live player';
  return (
    <header className={`match-header ${yourSide}-side`}>
      <div className="match-header-main">
        <div className="match-kicker">{queueLabel(liveGame.gameQueueConfigId)} · {liveGame.platform}</div>
        <h2>{searchedName}</h2>
        <div className="match-subline">
          <span>{liveGame.gameMode}</span>
          <span>{yourSide === 'blue' ? 'Blue side' : 'Red side'}</span>
          {patch ? <span>Patch {patch}</span> : null}
        </div>
      </div>
      <div className="match-header-stats" aria-label="Live match context">
        <HeaderStat label="Clock" value={matchClock(liveGame.gameStartTime, now)} />
        <HeaderStat label="Blue Avg" value={blueAverage.label} detail={blueAverage.detail} tone="blue" />
        <HeaderStat label="Red Avg" value={redAverage.label} detail={redAverage.detail} tone="red" />
      </div>
    </header>
  );
}

function HeaderStat({ label, value, detail, tone }: { label: string; value: string; detail?: string; tone?: TeamSide }) {
  return (
    <div className={`match-header-stat${tone ? ` ${tone}` : ''}`}>
      <span>{label}</span>
      <strong>{value}</strong>
      {detail ? <em>{detail}</em> : null}
    </div>
  );
}

function LaneHeader() {
  return (
    <div className="lane-header-row" aria-label="Matchup lanes">
      {roles.map((role) => (
        <div className="lane-header-cell" key={role}>{roleLabels[role] ?? role}</div>
      ))}
    </div>
  );
}

function LaneTabs({ selectedIndex, onSelect }: { selectedIndex: number; onSelect: (index: number) => void }) {
  return (
    <div className="mobile-lane-tabs" aria-label="Mobile lane selector">
      {roles.map((role, index) => (
        <button
          className={index === selectedIndex ? 'selected' : ''}
          key={role}
          onClick={() => onSelect(index)}
          type="button"
        >
          {roleLabels[role] ?? role}
        </button>
      ))}
    </div>
  );
}

function WinConditionPanel({
  analysis,
  yourSide,
  loading,
  error,
}: {
  analysis?: WinConditionAnalysisResponse;
  yourSide: TeamSide;
  loading: boolean;
  error?: string;
}) {
  if (loading) {
    return <section className="win-condition-panel win-condition-state">Win condition metrics loading...</section>;
  }
  if (error) {
    return <section className="win-condition-panel win-condition-state">Win condition metrics unavailable</section>;
  }
  if (!analysis) return null;

  return <WinConditionContent analysis={analysis} yourSide={yourSide} />;
}

function WinConditionContent({ analysis, yourSide }: { analysis: WinConditionAnalysisResponse; yourSide: TeamSide }) {
  const enemySide: TeamSide = yourSide === 'blue' ? 'red' : 'blue';
  const yourTeam = yourSide === 'blue' ? analysis.blue : analysis.red;
  const enemyTeam = enemySide === 'blue' ? analysis.blue : analysis.red;
  const yourMetrics = yourSide === 'blue' ? analysis.blueMatchups : analysis.redMatchups;
  const enemyMetrics = enemySide === 'blue' ? analysis.blueMatchups : analysis.redMatchups;
  const [selectedYourCondition, setSelectedYourCondition] = useState(yourTeam.primaryCondition);
  const [selectedEnemyCondition, setSelectedEnemyCondition] = useState(enemyTeam.primaryCondition);

  useEffect(() => {
    setSelectedYourCondition(yourTeam.primaryCondition);
    setSelectedEnemyCondition(enemyTeam.primaryCondition);
  }, [analysis, yourTeam.primaryCondition, enemyTeam.primaryCondition]);

  const selectedYourMetric = metricForPair(yourMetrics, selectedYourCondition, selectedEnemyCondition) ?? metricForCondition(yourMetrics, selectedYourCondition) ?? primaryMetric(yourMetrics);
  const selectedEnemyMetric = metricForPair(enemyMetrics, selectedEnemyCondition, selectedYourCondition) ?? metricForCondition(enemyMetrics, selectedEnemyCondition) ?? primaryMetric(enemyMetrics);

  return (
    <section className="win-condition-panel" aria-label="Win condition stats">
      <WinConditionSummaryCard
        title="Your Team's Win Condition"
        side={yourSide}
        team={yourTeam}
        metric={selectedYourMetric}
        metrics={yourMetrics}
        selectedCondition={selectedYourMetric?.condition ?? yourTeam.primaryCondition}
        opponentCondition={selectedEnemyMetric?.condition ?? enemyTeam.primaryCondition}
        onSelect={setSelectedYourCondition}
      />
      <WinConditionScriptPanel metric={selectedYourMetric} />
      <WinConditionLengthChart metric={selectedYourMetric} />
      <WinConditionEnemyCard
        side={enemySide}
        team={enemyTeam}
        metric={selectedEnemyMetric}
        metrics={enemyMetrics}
        opponentCondition={selectedYourMetric?.condition ?? yourTeam.primaryCondition}
        onSelect={setSelectedEnemyCondition}
      />
    </section>
  );
}

function WinConditionSummaryCard({
  title,
  side,
  team,
  metric,
  metrics,
  selectedCondition,
  opponentCondition,
  onSelect,
}: {
  title: string;
  side: TeamSide;
  team: WinConditionTeamProfile;
  metric?: WinConditionMetric;
  metrics?: WinConditionMetric[];
  selectedCondition?: string;
  opponentCondition?: string;
  onSelect?: (condition: string) => void;
}) {
  const condition = metric?.condition ?? team.primaryCondition;
  const rating = metric?.rating ?? team.primaryRating;
  const hasSample = Boolean(metric && metric.games > 0);
  const winRateValue = hasSample && metric ? `${metric.winRate.toFixed(2)}%` : '--';
  const strategyValue = hasSample && metric ? (metric.planLabel ?? planLabelFallback(metric)) : 'No sample';
  const gamesValue = hasSample && metric ? String(metric.games) : '0';
  const confidenceValue = hasSample && metric ? (metric.evidence?.level ?? evidenceLevelFallback(metric.games)) : 'No sample';
  return (
    <div className={`legacy-win-card ${side}`}>
      <h2>{title}</h2>
      <div className="legacy-win-images">
        <img className="legacy-condition-icon" src={conditionIconUrl(condition)} alt="" />
        <img className="legacy-rating-icon" src={ratingImageUrl(rating)} alt={rating} />
      </div>
      <div className="legacy-win-stat">
        <div className="legacy-win-stat-heading">
          <span>{condition}</span>
          <strong>{rating}</strong>
        </div>
        <div className="legacy-win-stat-grid">
          <WinConditionStatTile label="Win Rate" value={winRateValue} accent />
          <WinConditionStatTile label="Strategy" value={strategyValue} />
          <WinConditionStatTile label="Total Games" displayLabel="Games" value={gamesValue} />
          <WinConditionStatTile label="Confidence" value={confidenceValue} />
        </div>
      </div>
      <WinConditionProfileBars team={team} selectedCondition={condition} />
      {metrics && opponentCondition && onSelect ? (
        <WinConditionPlanSwitches
          label="Other Strategies"
          metrics={metrics}
          selectedCondition={selectedCondition ?? condition}
          opponentCondition={opponentCondition}
          primaryCondition={team.primaryCondition}
          onSelect={onSelect}
        />
      ) : null}
    </div>
  );
}

function WinConditionStatTile({ label, displayLabel, value, accent = false }: { label: string; displayLabel?: string; value: string; accent?: boolean }) {
  return (
    <span className={`legacy-win-stat-tile${accent ? ' accent' : ''}`} aria-label={`${label}: ${value}`}>
      <small>{displayLabel ?? label}</small>
      <strong>{value}</strong>
    </span>
  );
}

function WinConditionProfileBars({ team, selectedCondition }: { team: WinConditionTeamProfile; selectedCondition: string }) {
  return (
    <div className="win-profile-bars" aria-label="Team win condition profile">
      {team.axes.map((axis) => (
        <div className={`win-profile-row${axis.label === selectedCondition ? ' selected' : ''}`} key={axis.key}>
          <span className="win-profile-name">{axis.label}</span>
          <span className="win-profile-track">
            <span className="win-profile-fill" style={{ width: `${Math.min(100, (axis.score / 25) * 100)}%` }} />
          </span>
          <span className="win-profile-rating">{axis.rating}</span>
        </div>
      ))}
    </div>
  );
}

function WinConditionPlanSwitches({
  label,
  metrics,
  selectedCondition,
  opponentCondition,
  primaryCondition,
  axisOrder,
  allStrategies = false,
  compact = false,
  maxItems = 3,
  showStats = true,
  onSelect,
}: {
  label: string;
  metrics: WinConditionMetric[];
  selectedCondition: string;
  opponentCondition: string;
  primaryCondition: string;
  axisOrder?: string[];
  allStrategies?: boolean;
  compact?: boolean;
  maxItems?: number;
  showStats?: boolean;
  onSelect: (condition: string) => void;
}) {
  const alternatives = allStrategies
    ? allPlanSwitchMetrics(metrics, opponentCondition, axisOrder, selectedCondition)
    : planSwitchMetrics(metrics, selectedCondition, opponentCondition, primaryCondition);
  return (
    <div className={`strategy-switcher${compact ? ' compact' : ''}`}>
      <span className="strategy-switcher-title">{label}</span>
      <div className="strategy-switch-list">
        {alternatives.length > 0 ? (
          alternatives.slice(0, maxItems).map((metric) => (
            <button
              className={`strategy-switch${metric.condition === selectedCondition ? ' selected' : ''}`}
              key={`${metric.condition}-${metric.opponentCondition}`}
              type="button"
              onClick={() => onSelect(metric.condition)}
              aria-current={metric.condition === selectedCondition ? 'true' : undefined}
              aria-label={`Show ${label} ${metric.condition}`}
            >
              <img src={conditionIconUrl(metric.condition)} alt="" />
              <span>
                <strong>{metric.condition} {metric.rating}</strong>
                {showStats ? (
                  <em>{metric.condition === primaryCondition ? 'Primary · ' : ''}{metric.games > 0 ? `${metric.winRate.toFixed(0)}% · ${metric.games}g` : 'No sample'}</em>
                ) : null}
              </span>
            </button>
          ))
        ) : (
          <span className="strategy-switch-empty">{allStrategies ? 'No strategies available' : 'No other strong strategies'}</span>
        )}
      </div>
    </div>
  );
}

function WinConditionScriptPanel({ metric }: { metric?: WinConditionMetric }) {
  const script = metric?.script;
  return (
    <div className="legacy-stats-section match-read-section">
      <h2>Match Read</h2>
      <div className="match-read-copy">
        {script ? (
          <>
            <WinConditionPairStrip metric={metric} />
            <div className="match-read-headline">
              <strong>{script.headline}</strong>
              <span>{planPairRead(metric)}</span>
              <div className="match-read-evidence-row" aria-label="Evidence summary">
                <EvidencePill metric={metric} />
                <span>{metric?.games.toLocaleString() ?? 0} games</span>
                {metric?.evidence?.wilsonLow || metric?.evidence?.wilsonHigh ? (
                  <span>{metric.evidence.wilsonLow.toFixed(1)}-{metric.evidence.wilsonHigh.toFixed(1)} likely range</span>
                ) : null}
              </div>
            </div>
            <p className="match-read-primary">{script.playerRead}</p>
            <div className="match-read-grid">
              <ReadBlock label="Play toward" text={script.modeRead} />
              <ReadBlock label="Watch for" text={script.matchup} />
              <ReadBlock label="Timing" text={script.timingRead} />
              <ReadBlock label="Evidence" text={metric.evidence?.summary ?? script.sampleRead} />
            </div>
            {script.cautionRead ? <p className="match-read-caution">{script.cautionRead}</p> : null}
            <em>{script.sampleRead}</em>
          </>
        ) : (
          <p>Select a win condition pairing to see the match read.</p>
        )}
      </div>
    </div>
  );
}

function WinConditionPairStrip({ metric }: { metric: WinConditionMetric }) {
  return (
    <div className="match-read-pair-strip" aria-label="Selected win condition pairing">
      <span className="pair-side your">
        <img src={conditionIconUrl(metric.condition)} alt="" />
        <strong>Your {metric.condition} {metric.rating}</strong>
      </span>
      <em>vs</em>
      <span className="pair-side enemy">
        <img src={conditionIconUrl(metric.opponentCondition)} alt="" />
        <strong>Enemy {metric.opponentCondition} {metric.opponentRating}</strong>
      </span>
    </div>
  );
}

function EvidencePill({ metric }: { metric?: WinConditionMetric }) {
  const direction = metric?.evidence?.direction ?? 'unknown';
  const score = metric?.evidence?.score ?? 0;
  const level = metric?.evidence?.level ?? 'No sample';
  return (
    <span className={`evidence-pill ${direction}`}>
      Confidence: {level}{score > 0 ? ` ${score.toFixed(0)}/100` : ''}
    </span>
  );
}

function ReadBlock({ label, text }: { label: string; text: string }) {
  return (
    <div className="match-read-block">
      <span>{label}</span>
      <p>{text}</p>
    </div>
  );
}

function WinConditionLengthChart({ metric }: { metric?: WinConditionMetric }) {
  const buckets = metric?.buckets ?? [];
  const points = buckets.map((bucket, index) => ({
    x: buckets.length <= 1 ? 50 : 12 + (index * 76) / (buckets.length - 1),
    y: chartY(bucket.winRate),
    bucket,
  })).filter((point) => point.bucket.games > 0);
  const pointString = points.map((point) => `${point.x},${point.y}`).join(' ');
  const areaPath = chartAreaPath(points);
  return (
    <div className="legacy-stats-section chart-section">
      <h2>Winrate By Game Length</h2>
      <div className="chart-shell">
        <div className="chart-plot">
          <svg className="winrate-chart" viewBox="0 0 100 100" role="img" aria-label="Winrate by game length from 35% to 65%" preserveAspectRatio="none">
            <defs>
              <linearGradient id="durationWinrateArea" x1="0" x2="0" y1="0" y2="1">
                <stop offset="0%" stopColor="#7df4dd" stopOpacity="0.28" />
                <stop offset="58%" stopColor="#4bc0c0" stopOpacity="0.11" />
                <stop offset="100%" stopColor="#ff7979" stopOpacity="0.08" />
              </linearGradient>
            </defs>
            <line className="chart-gridline" x1="10" y1={chartY(65)} x2="92" y2={chartY(65)} />
            <line className="chart-gridline chart-baseline" x1="10" y1={chartY(50)} x2="92" y2={chartY(50)} />
            <line className="chart-gridline" x1="10" y1={chartY(35)} x2="92" y2={chartY(35)} />
            {areaPath ? <path className="chart-area" d={areaPath} /> : null}
            {pointString ? <polyline className="chart-line" points={pointString} /> : null}
          </svg>
          <div className="chart-y-axis" aria-hidden="true">
            <span style={{ top: `${chartY(65)}%` }}>65%</span>
            <span className="chart-y-axis-baseline" style={{ top: `${chartY(50)}%` }}>50%</span>
            <span style={{ top: `${chartY(35)}%` }}>35%</span>
          </div>
          {points.map((point) => (
            <span
              className={`chart-marker ${chartPointClass(point.bucket.winRate)}`}
              key={point.bucket.bucket}
              style={{ left: `${point.x}%`, top: `${point.y}%` }}
              title={`${point.bucket.bucket}: ${point.bucket.winRate.toFixed(1)}% over ${point.bucket.games} games`}
            >
              <span className="chart-marker-value">{point.bucket.winRate.toFixed(0)}%</span>
            </span>
          ))}
          {!points.length ? <div className="chart-no-samples">No duration samples</div> : null}
        </div>
        <div className="chart-labels">
          {buckets.map((bucket) => (
            <span
              className={`${bucket.meetsMinGames ? '' : 'thin-sample'} ${bucket.games > 0 ? chartPointClass(bucket.winRate) : ''}`.trim()}
              key={bucket.bucket}
            >
              <b>{bucket.bucket}</b>
              <strong>{bucket.games > 0 ? `${bucket.winRate.toFixed(0)}%` : '--'}</strong>
            </span>
          ))}
        </div>
      </div>
    </div>
  );
}

const chartMinWinRate = 35;
const chartMaxWinRate = 65;
const chartTop = 8;
const chartBottom = 92;

function chartY(winRate: number) {
  const clamped = Math.max(chartMinWinRate, Math.min(chartMaxWinRate, winRate));
  const progress = (clamped - chartMinWinRate) / (chartMaxWinRate - chartMinWinRate);
  return chartBottom - progress * (chartBottom - chartTop);
}

function chartAreaPath(points: { x: number; y: number }[]) {
  if (!points.length) return '';
  const first = points[0];
  const last = points[points.length - 1];
  const line = points.map((point) => `L ${point.x} ${point.y}`).join(' ');
  return `M ${first.x} ${chartBottom} ${line} L ${last.x} ${chartBottom} Z`;
}

function chartPointClass(winRate: number) {
  if (winRate > 50) return 'chart-point-favorable';
  if (winRate < 50) return 'chart-point-unfavorable';
  return 'chart-point-even';
}

function WinConditionEnemyCard({
  side,
  team,
  metric,
  metrics,
  opponentCondition,
  onSelect,
}: {
  side: TeamSide;
  team: WinConditionTeamProfile;
  metric?: WinConditionMetric;
  metrics: WinConditionMetric[];
  opponentCondition: string;
  onSelect: (condition: string) => void;
}) {
  const condition = metric?.condition ?? team.primaryCondition;
  const rating = metric?.rating ?? team.primaryRating;
  return (
    <div className={`legacy-win-card enemy ${side}`}>
      <h2>Enemy Team's Win Condition</h2>
      <div className="legacy-win-images">
        <img className="legacy-condition-icon" src={conditionIconUrl(condition)} alt="" />
        <img className="legacy-rating-icon" src={ratingImageUrl(rating)} alt={rating} />
      </div>
      <WinConditionProfileBars team={team} selectedCondition={condition} />
      <div className="enemy-strategy-note">
        <strong>Adjust The Read</strong>
        <span>If the enemy is clearly playing through another strategy, select it below to update the matchup context.</span>
      </div>
      <WinConditionPlanSwitches
        label="Enemy Strategies"
        metrics={metrics}
        selectedCondition={condition}
        opponentCondition={opponentCondition}
        primaryCondition={team.primaryCondition}
        axisOrder={team.axes.map((axis) => axis.label)}
        allStrategies
        compact
        maxItems={4}
        showStats={false}
        onSelect={onSelect}
      />
    </div>
  );
}

function FocusedBuildPanel({
  selection,
  champions,
  items,
  matchupItemSlots,
  championItemSlots,
  loading,
  selectedParticipantKey,
  onSelectParticipant,
  selectedOpponentKey,
  onSelectOpponent,
}: {
  selection?: FocusedBuildSelection;
  champions?: ChampionData;
  items?: ItemData;
  matchupItemSlots: AnalyticsItemSlot[];
  championItemSlots: AnalyticsItemSlot[];
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
  const matchupSample = buildSampleQuality(highestSlotSample(matchupItemSlots));
  const championSample = buildSampleQuality(highestSlotSample(championItemSlots));
  const matchupScopeLabel = buildScopeLabel(matchupItemSlots);
  const championScopeLabel = buildScopeLabel(championItemSlots);
  const activeParticipantKey = selectedParticipantKey || selection.participantKey;
  const activeOpponentKey = selectedOpponentKey || selection.opponentKey;

  return (
    <section className={`focused-build-panel ${selection.side}`} aria-label="Focused build matchup">
      <div className="focused-build-header">
        <div className="focused-build-player">
          {championUrl ? <img src={championUrl} alt={champion?.name ?? 'Champion'} /> : null}
          <span>
            <small>{roleLabels[selection.role] ?? selection.role}</small>
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
      <div className="focused-build-results">
        <BuildResultCard
          title="Matchup Build"
          description={`${champion?.name ?? selection.participant.championId} vs ${opponentChampion?.name ?? selection.opponent.championId}`}
          sample={matchupSample}
          scopeLabel={matchupScopeLabel}
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
          scopeLabel={championScopeLabel}
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
              <small>{roleLabels[option.role] ?? option.role}</small>
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
  scopeLabel,
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
  scopeLabel: string;
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
        </span>
        <div>
          <b className={`build-sample-chip ${sample.tone}`}>{sample.label}</b>
          <small className="build-min-sample">{minGames}+ games/item</small>
          {scopeLabel ? <small className="build-scope-label">{scopeLabel}</small> : null}
        </div>
      </header>
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

function LiveChampionCard({
  participant,
  index,
  role,
  side,
  champions,
  spells,
  runes,
  dragging,
  dropTarget,
  onDragStart,
  onDragOver,
  onDrop,
  onDragEnd,
  mobileActive,
}: {
  participant: LiveParticipant;
  index: number;
  role: string;
  side: TeamSide;
  champions?: ChampionData;
  spells?: SummonerSpellData;
  runes?: RuneData;
  dragging: boolean;
  dropTarget: boolean;
  onDragStart: () => void;
  onDragOver: (event: DragEvent<HTMLElement>) => void;
  onDrop: (event: DragEvent<HTMLElement>) => void;
  onDragEnd: () => void;
  mobileActive: boolean;
}) {
  const champion = championByKey(champions, participant.championId);
  const championName = champion?.name ?? String(participant.championId);
  const championUrl = championImageUrl(champions, participant.championId);
  const championBackdropUrl = championSplashUrl(champions, participant.championId);
  const keystoneId = participant.perks?.perkIds?.[0];
  const secondaryStyleId = participant.perks?.perkSubStyle;
  const playerName = participant.riotId || participant.summonerName || 'Unknown player';
  const comfortFlags = comfortFlagsForParticipant(participant);

  return (
    <article
      className={`player-card ${side}-style${dragging ? ' dragging' : ''}${dropTarget ? ' drop-target' : ''}${mobileActive ? ' mobile-active' : ' mobile-hidden'}`}
      draggable
      onDragStart={(event) => {
        event.dataTransfer.effectAllowed = 'move';
        event.dataTransfer.setData('text/plain', `${side}:${index}`);
        onDragStart();
      }}
      onDragOver={onDragOver}
      onDrop={onDrop}
      onDragEnd={onDragEnd}
      title="Drag to reorder matchup slots"
    >
      {championBackdropUrl ? <img className="player-card-backdrop" src={championBackdropUrl} alt="" aria-hidden="true" /> : null}
      <GripVertical className="card-drag-handle" size={16} aria-hidden="true" />
      <div className="card-topline">
        {championUrl ? <img className="profile-picture" src={championUrl} alt={championName} /> : <div className="profile-picture profile-fallback">{participant.championId}</div>}
        <img className="ranked-card-icon" src={rankIconUrl(participant.rank)} alt={participant.rank ? rankLabel(participant.rank) : 'Rank unavailable'} />
        <div className="player-title">
          <div className="player-role-chip">{roleLabels[role] ?? role ?? 'Role'}</div>
          <div className="summoner-name" title={playerName}>{playerName}</div>
          <div className="champion-name">{championName}</div>
        </div>
      </div>
      {comfortFlags.length > 0 ? (
        <div className="card-context-row">
          {comfortFlags.map((flag) => (
            <span className={`comfort-flag ${flag.tone}`} key={flag.label}>{flag.label}</span>
          ))}
        </div>
      ) : null}
      <div className="player-stat-columns">
        <ChampionRecordBlock stats={participant.championStats} />
        <RankRecord rank={participant.rank} />
      </div>
      <div className="card-loadout">
        <div className="loadout-group">
          <span>Spells</span>
          <div className="summoner-spells">
            {[participant.spell1Id, participant.spell2Id].map((spellId) => {
              const imageUrl = summonerSpellImageUrl(spells, spellId);
              const name = summonerSpellName(spells, spellId);
              return imageUrl ? <img key={spellId} src={imageUrl} alt={name} title={name} /> : <span className="spell-pill" key={spellId}>{spellId}</span>;
            })}
          </div>
        </div>
        <div className="loadout-group">
          <span>Runes</span>
          <div className="runes">
            <RuneIcon runes={runes} runeId={keystoneId} />
            <RuneStyleIcon runes={runes} styleId={secondaryStyleId} />
          </div>
        </div>
      </div>
    </article>
  );
}

function RankRecord({ rank }: { rank?: RankedRecord }) {
  if (!rank) {
    return (
      <div className="rank-info rank-missing">
        <div className="stat-block-title">Ranked</div>
        <div className="stat-chip-grid">
          <StatChip label="Rank" value="Unavailable" primary />
          <StatChip label="Winrate" value="--" />
          <StatChip label="Games" value="--" />
        </div>
      </div>
    );
  }
  return (
    <div className={`rank-info${rank.rankAvailable === false ? ' rank-unranked' : ''}`}>
      <div className="stat-block-title">Ranked</div>
      <div className="stat-chip-grid">
        <StatChip label="Rank" value={rankLabel(rank)} primary />
        <StatChip label="Winrate" value={`${rank.winRate.toFixed(1)}%`} />
        <StatChip label="Games" value={String(rank.totalGames)} />
      </div>
    </div>
  );
}

function ChampionRecordBlock({ stats }: { stats?: ChampionRecord }) {
  if (!stats) {
    return (
      <div className="champion-performance stats-missing">
        <div className="stat-block-title">Champion</div>
        <div className="stat-chip-grid">
          <StatChip label="Games" value="0" primary />
          <StatChip label="Champ WR" value="--" />
          <StatChip label="KDA" value="--" />
        </div>
      </div>
    );
  }
  return (
    <div className="champion-performance">
      <div className="stat-block-title">Champion</div>
      <div className="stat-chip-grid">
        <StatChip label="Games" value={String(stats.games)} primary />
        <StatChip label="Champ WR" value={`${stats.winRate.toFixed(1)}%`} />
        <StatChip label="KDA" value={stats.kda.toFixed(2)} />
        <StatChip label="Avg K/D/A" value={`${stats.avgKills.toFixed(1)} / ${stats.avgDeaths.toFixed(1)} / ${stats.avgAssists.toFixed(1)}`} wide />
      </div>
    </div>
  );
}

function StatChip({ label, value, primary, wide }: { label: string; value: string; primary?: boolean; wide?: boolean }) {
  return (
    <span className={`stat-chip${primary ? ' primary' : ''}${wide ? ' wide' : ''}`}>
      <strong>{value}</strong>
      <em>{label}</em>
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

  const bestBySlot = [1, 2, 3, 4, 5, 6].map((slot) => ({
    slot,
    row: itemSlots.find((candidate) => candidate.itemSlot === slot),
  }));

  return (
    <div className={`build-side ${side}`}>
      {bestBySlot.map(({ slot, row }) => (
        row ? <ItemSlotLine key={`${row.itemSlot}-${row.itemId}`} row={row} items={items} /> : <MissingItemSlotLine key={slot} slot={slot} />
      ))}
    </div>
  );
}

function ItemSlotLine({ row, items }: { row: AnalyticsItemSlot; items?: ItemData }) {
  const itemId = String(row.itemId);
  const imageUrl = itemImageUrl(items, itemId);
  const name = itemName(items, itemId);
  return (
    <div className={`item-slot-column${row.games < 5 ? ' low-sample-item' : ''}`} title={`${ordinal(row.itemSlot)} item: ${name}. ${row.winRate.toFixed(1)}% over ${row.games} games.`}>
      <span className="item-slot-number">{ordinal(row.itemSlot)}</span>
      {imageUrl ? <img src={imageUrl} alt={name} title={name} /> : <span className="item-pill">{row.itemId}</span>}
      <div className="item-slot-stats">
        <strong>{row.winRate.toFixed(1)}%</strong>
        <span>{row.games}g{row.games < 5 ? ' · thin' : ''}</span>
      </div>
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

function RuneIcon({ runes, runeId }: { runes?: RuneData; runeId?: number }) {
  const imageUrl = runeImageUrl(runes, runeId);
  const name = runeName(runes, runeId);
  return imageUrl ? <img src={imageUrl} alt={name} title={name} /> : <span className="rune-fallback">{runeId ?? '?'}</span>;
}

function RuneStyleIcon({ runes, styleId }: { runes?: RuneData; styleId?: number }) {
  const styleName = runeStyleName(runes, styleId);
  const style = runes?.data.find((candidate) => candidate.id === styleId);
  return style?.icon ? <img src={`https://ddragon.leagueoflegends.com/cdn/img/${style.icon}`} alt={styleName} title={styleName} /> : <span className="rune-fallback">{styleId ?? '?'}</span>;
}

function buildFilters(participant: LiveParticipant, opponent: LiveParticipant, role: string, patch?: string): BuildFilters {
  return {
    championId: participant.championId,
    opponentChampionId: opponent.championId,
    itemContext: itemContextForParticipant(participant, role),
    patch,
    minGames: BUILD_MATCHUP_MIN_GAMES,
    limit: 6,
    fallback: Boolean(patch),
  };
}

function championBuildFiltersFor(participant: LiveParticipant, role: string, patch?: string): BuildFilters {
  return {
    championId: participant.championId,
    itemContext: itemContextForParticipant(participant, role),
    patch,
    minGames: BUILD_BASELINE_MIN_GAMES,
    limit: 6,
    fallback: Boolean(patch),
  };
}

function focusedBuildSelection(
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

function participantDisplayName(participant: LiveParticipant) {
  return participant.riotId || participant.summonerName || 'Unknown player';
}

function comfortFlagsForParticipant(participant: LiveParticipant) {
  const flags: { label: string; tone: string }[] = [];
  const stats = participant.championStats;
  if (!stats || stats.games <= 0) {
    flags.push({ label: 'No champ sample', tone: 'thin' });
  } else if (stats.games < 5) {
    flags.push({ label: 'Low champ sample', tone: 'thin' });
  } else if (stats.games >= 20 && stats.winRate >= 55) {
    flags.push({ label: 'Champion comfort', tone: 'good' });
  } else if (stats.games >= 20 && stats.winRate <= 45) {
    flags.push({ label: 'Rough champ sample', tone: 'warn' });
  }

  const rank = participant.rank;
  if (!rank || rank.rankAvailable === false) {
    flags.push({ label: 'Rank pending', tone: 'thin' });
  } else if (rank.totalGames >= 30 && rank.winRate >= 55) {
    flags.push({ label: 'Strong ranked form', tone: 'good' });
  }

  return flags.slice(0, 2);
}

function highestSlotSample(itemSlots: AnalyticsItemSlot[]) {
  return itemSlots.reduce((max, row) => Math.max(max, row.games), 0);
}

function buildSampleQuality(samples: number) {
  if (samples <= 0) return { label: 'No samples', tone: 'none' };
  if (samples < 5) return { label: `${samples} sample${samples === 1 ? '' : 's'} · thin`, tone: 'thin' };
  if (samples < 15) return { label: `${samples} samples · early`, tone: 'early' };
  if (samples < 50) return { label: `${samples} samples · useful`, tone: 'useful' };
  return { label: `${samples} samples · strong`, tone: 'strong' };
}

function buildScopeLabel(itemSlots: AnalyticsItemSlot[]) {
  const labels = [...new Set(itemSlots.map((row) => row.sampleScopeLabel).filter(Boolean))];
  if (!labels.length) return '';
  if (labels.length === 1) return labels[0] ?? '';
  return 'Mixed fallback samples';
}

function matchClock(gameStartTime: number, now: number) {
  if (!gameStartTime) return '--:--';
  const elapsedSeconds = Math.max(0, Math.floor((now - gameStartTime) / 1000));
  const minutes = Math.floor(elapsedSeconds / 60);
  const seconds = elapsedSeconds % 60;
  return `${minutes}:${seconds.toString().padStart(2, '0')}`;
}

function averageRankLabel(team: LiveParticipant[]) {
  const values = team.map(rankValue).filter((value): value is number => value !== undefined);
  if (!values.length) {
    return { label: 'Unknown', detail: `0/${team.length || 5} ranked` };
  }
  const average = values.reduce((sum, value) => sum + value, 0) / values.length;
  return {
    label: labelForRankValue(average),
    detail: `${values.length}/${team.length || 5} ranked`,
  };
}

const rankTierValues: Record<string, number> = {
  IRON: 0,
  BRONZE: 4,
  SILVER: 8,
  GOLD: 12,
  PLATINUM: 16,
  EMERALD: 20,
  DIAMOND: 24,
  MASTER: 28,
  GRANDMASTER: 30,
  CHALLENGER: 32,
};

const rankDivisionValues: Record<string, number> = {
  IV: 0,
  III: 1,
  II: 2,
  I: 3,
};

function rankValue(participant: LiveParticipant) {
  const rank = participant.rank;
  if (!rank || rank.rankAvailable === false || !rank.tier) return undefined;
  const tierValue = rankTierValues[rank.tier.toUpperCase()];
  if (tierValue === undefined) return undefined;
  if (tierValue >= rankTierValues.MASTER) {
    return tierValue + Math.min(1.9, Math.max(0, rank.leaguePoints) / 500);
  }
  const division = rank.division || rank.rank || 'IV';
  return tierValue + (rankDivisionValues[division.toUpperCase()] ?? 0) + Math.min(0.99, Math.max(0, rank.leaguePoints) / 100);
}

function labelForRankValue(value: number) {
  if (value >= rankTierValues.CHALLENGER) return 'Challenger';
  if (value >= rankTierValues.GRANDMASTER) return 'Grandmaster';
  if (value >= rankTierValues.MASTER) return 'Master';
  const tiers = ['IRON', 'BRONZE', 'SILVER', 'GOLD', 'PLATINUM', 'EMERALD', 'DIAMOND'];
  const tierIndex = Math.max(0, Math.min(tiers.length - 1, Math.floor(value / 4)));
  const divisionOffset = Math.max(0, Math.min(3, Math.round(value - tierIndex * 4)));
  const division = ['IV', 'III', 'II', 'I'][divisionOffset];
  return `${titleCase(tiers[tierIndex])} ${division}`;
}

function titleCase(value: string) {
  return value.charAt(0).toUpperCase() + value.slice(1).toLowerCase();
}

function ordinal(value: number) {
  if (value === 1) return '1st';
  if (value === 2) return '2nd';
  if (value === 3) return '3rd';
  return `${value}th`;
}

function moveParticipantToIndex(team: LiveParticipant[], fromIndex: number, toIndex: number) {
  if (fromIndex === toIndex || fromIndex < 0 || toIndex < 0 || fromIndex >= team.length || toIndex >= team.length) {
    return team;
  }
  const next = [...team];
  const [current] = next.splice(fromIndex, 1);
  next.splice(toIndex, 0, current);
  return next;
}

function orderTeam(team: LiveParticipant[], roleRates: RoleRateMap = new Map()) {
  if (team.length !== 5) return team;
  const roleAssignments = new Map<string, LiveParticipant>();
  const lockedParticipants = new Set<LiveParticipant>();
  const smiters = team.filter(hasSmite);
  if (smiters.length === 1) {
    roleAssignments.set('JUNGLE', smiters[0]);
    lockedParticipants.add(smiters[0]);
  }

  const openRoles = roles.filter((role) => !roleAssignments.has(role));
  const openParticipants = team.filter((participant) => !lockedParticipants.has(participant));
  const best = bestRoleAssignment(openParticipants, openRoles, team, roleRates);
  best.forEach((participant, role) => roleAssignments.set(role, participant));

  return roles.map((role) => roleAssignments.get(role)).filter((participant): participant is LiveParticipant => Boolean(participant));
}

function bestRoleAssignment(participants: LiveParticipant[], openRoles: string[], originalTeam: LiveParticipant[], roleRates: RoleRateMap) {
  const best = {
    score: Number.NEGATIVE_INFINITY,
    assignments: new Map<string, LiveParticipant>(),
  };
  const used = new Set<number>();
  const current = new Map<string, LiveParticipant>();

  function visit(roleIndex: number, score: number) {
    if (roleIndex >= openRoles.length) {
      if (score > best.score) {
        best.score = score;
        best.assignments = new Map(current);
      }
      return;
    }
    const role = openRoles[roleIndex];
    for (let index = 0; index < participants.length; index++) {
      if (used.has(index)) continue;
      const participant = participants[index];
      used.add(index);
      current.set(role, participant);
      visit(roleIndex + 1, score + roleScore(participant, role, originalTeam, roleRates));
      current.delete(role);
      used.delete(index);
    }
  }

  visit(0, 0);
  return best.assignments;
}

function roleScore(participant: LiveParticipant, role: string, originalTeam: LiveParticipant[], roleRates: RoleRateMap) {
  const ratesForChampion = roleRates.get(participant.championId);
  const rate = ratesForChampion?.get(role);
  const sampleWeight = rate ? Math.min(1, Math.sqrt(rate.totalGames) / 5) : 0;
  const popularityScore = rate ? rate.pickRate * sampleWeight : 0;
  const originalIndex = originalTeam.indexOf(participant);
  const targetIndex = roles.indexOf(role);
  const orderTiebreaker = Math.max(0, 5 - Math.abs(originalIndex - targetIndex)) / 1000;
  const smiteScore = hasSmite(participant) && role === 'JUNGLE' ? 500 : 0;
  const smitePenalty = hasSmite(participant) && role !== 'JUNGLE' ? -500 : 0;
  return popularityScore + smiteScore + smitePenalty + orderTiebreaker;
}

function hasSmite(participant: LiveParticipant) {
  return participant.spell1Id === 11 || participant.spell2Id === 11;
}

function uniqueChampionIds(participants: LiveParticipant[]) {
  return [...new Set(participants.map((participant) => participant.championId).filter(Boolean))].sort((a, b) => a - b);
}

function teamChampionIds(participants: LiveParticipant[]) {
  return participants.map((participant) => participant.championId).filter(Boolean);
}

function patchBucketFromVersion(version?: string) {
  const match = version?.match(/^(\d+\.\d+)/);
  return match?.[1];
}

function buildRoleRateMap(rows: ChampionRoleRate[] = []): RoleRateMap {
  const map: RoleRateMap = new Map();
  rows.forEach((row) => {
    const championRates = map.get(row.championId) ?? new Map<string, ChampionRoleRate>();
    championRates.set(row.role, row);
    map.set(row.championId, championRates);
  });
  return map;
}

function metricForPair(metrics: WinConditionMetric[], condition: string, opponentCondition: string) {
  return metrics.find((metric) => metric.condition === condition && metric.opponentCondition === opponentCondition);
}

function metricForCondition(metrics: WinConditionMetric[], condition: string) {
  return metrics.find((metric) => metric.condition === condition);
}

function primaryMetric(metrics: WinConditionMetric[]) {
  return metrics.find((metric) => metric.primary) ?? metrics[0];
}

function sortedAlternativeMetrics(metrics: WinConditionMetric[]) {
  return [...metrics].sort((a, b) => {
    if (b.winRate !== a.winRate) return b.winRate - a.winRate;
    return b.games - a.games;
  });
}

function uniqueConditionMetrics(metrics: WinConditionMetric[]) {
  const seen = new Set<string>();
  return metrics.filter((metric) => {
    if (seen.has(metric.condition)) return false;
    seen.add(metric.condition);
    return true;
  });
}

function planSwitchMetrics(metrics: WinConditionMetric[], selectedCondition: string, opponentCondition: string, primaryCondition: string) {
  const exactAlternatives = uniqueConditionMetrics(sortedAlternativeMetrics(metrics).filter((metric) => (
    metric.opponentCondition === opponentCondition
    && metric.condition !== selectedCondition
    && isPlayerFacingPlan(metric)
  )));
  if (selectedCondition === primaryCondition) {
    return exactAlternatives.slice(0, 3);
  }

  const primaryReturn = exactAlternatives.find((metric) => metric.condition === primaryCondition)
    ?? metrics.find((metric) => metric.condition === primaryCondition && metric.opponentCondition === opponentCondition)
    ?? metrics.find((metric) => metric.condition === primaryCondition);
  if (!primaryReturn) {
    return exactAlternatives.slice(0, 3);
  }

  const withoutPrimary = exactAlternatives.filter((metric) => metric.condition !== primaryCondition);
  return [primaryReturn, ...withoutPrimary].slice(0, 3);
}

function allPlanSwitchMetrics(metrics: WinConditionMetric[], opponentCondition: string, axisOrder: string[] = [], excludedCondition?: string) {
  const byCondition = new Map<string, WinConditionMetric>();
  metrics
    .filter((metric) => metric.opponentCondition === opponentCondition && metric.condition !== excludedCondition)
    .forEach((metric) => {
      if (!byCondition.has(metric.condition)) {
        byCondition.set(metric.condition, metric);
      }
    });

  const ordered = axisOrder
    .map((condition) => byCondition.get(condition))
    .filter((metric): metric is WinConditionMetric => Boolean(metric));
  const remaining = [...byCondition.values()]
    .filter((metric) => !axisOrder.includes(metric.condition))
    .sort((a, b) => ratingRank(b.rating) - ratingRank(a.rating));
  return [...ordered, ...remaining];
}

function isPlayerFacingPlan(metric: WinConditionMetric) {
  const role = metric.planRole?.toLowerCase();
  if (metric.primary || role === 'primary') return true;
  if (role === 'co-primary' || role === 'strong-secondary') return true;
  if (role === 'secondary') {
    return isRelevantSecondaryPlan(metric);
  }
  if (!role) {
    return metric.primary || isRelevantSecondaryPlan(metric);
  }
  return false;
}

function isRelevantSecondaryPlan(metric: WinConditionMetric) {
  const closeToPrimary = metric.deltaFromPrimary !== undefined && metric.deltaFromPrimary <= 5;
  return closeToPrimary || ratingRank(metric.rating) >= ratingRank('B-');
}

function ratingRank(rating: string) {
  const ratings = ['D-', 'D', 'D+', 'C-', 'C', 'C+', 'B-', 'B', 'B+', 'A-', 'A', 'A+', 'S-', 'S', 'S+'];
  const index = ratings.indexOf(rating);
  return index >= 0 ? index : -1;
}

function evidenceLevelFallback(games: number) {
  if (games <= 0) return 'No sample';
  if (games < 25) return 'Thin';
  if (games < 100) return 'Early';
  if (games < 400) return 'Moderate';
  if (games < 1600) return 'Strong';
  return 'Very strong';
}

function planLabelFallback(metric?: WinConditionMetric) {
  if (!metric) return 'Unknown';
  return metric.primary ? 'Primary' : 'Alternative';
}

function planPairRead(metric: WinConditionMetric) {
  const ownPlan = metric.planLabel ?? planLabelFallback(metric);
  const enemyPlan = metric.opponentPlanLabel ?? (metric.opponentPrimary ? 'Primary' : 'Alternative');
  return `Strategy context: your ${metric.condition} is ${ownPlan.toLowerCase()} into the enemy ${metric.opponentCondition} ${enemyPlan.toLowerCase()}.`;
}

function livePlayerSide(liveGame: LiveGame): TeamSide {
  const participant = liveGame.participants.find((candidate) => idsMatch(candidate.puuid, liveGame.puuid));
  return participant?.teamId === 200 ? 'red' : 'blue';
}

function sameParticipantIdentity(participant: LiveParticipant, target: LiveParticipant) {
  return idsMatch(participant.puuid, target.puuid) || idsMatch(participant.summonerId, target.summonerId);
}

function idsMatch(left?: string, right?: string) {
  const normalizedLeft = left?.trim();
  const normalizedRight = right?.trim();
  return Boolean(normalizedLeft && normalizedRight && normalizedLeft === normalizedRight);
}

function participantKey(participant: LiveParticipant, index: number) {
  return `${participant.teamId}-${participant.summonerId ?? participant.riotId ?? participant.championId}-${index}`;
}

function queueLabel(queueId: number) {
  if (queueId === 420) return 'Ranked Solo/Duo';
  if (queueId === 440) return 'Ranked Flex';
  if (queueId === 400) return 'Normal Draft';
  if (queueId === 430) return 'Normal Blind';
  return `Queue ${queueId}`;
}

function conditionIconUrl(condition: string) {
  return `/images/win_condition_icons/${condition}.png`;
}

function ratingImageUrl(rating: string) {
  return `/images/win_condition_ratings/${rating}.png`;
}
