import { useQueries, useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useState, type DragEvent } from 'react';
import type { AnalyticsItemSlot, BuildFilters, ChampionData, ChampionRecord, ChampionRoleRate, ItemData, LiveGame, LiveParticipant, RankedRecord, RuneData, SummonerSpellData, WinConditionAnalysisResponse, WinConditionMetric, WinConditionTeamProfile } from '../api/types';
import { getChampionRoleRates, getItemSlots, getWinConditionAnalysis } from '../api/client';
import {
  championByKey,
  championImageUrl,
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

type Props = {
  liveGame: LiveGame;
  champions?: ChampionData;
  items?: ItemData;
  spells?: SummonerSpellData;
  runes?: RuneData;
};

type StatRequest = {
  key: string;
  filters: BuildFilters;
};

type TeamSide = 'blue' | 'red';

type DraggedCard = {
  side: TeamSide;
  index: number;
};

type RoleRateMap = Map<number, Map<string, ChampionRoleRate>>;

export function LiveMatchups({ liveGame, champions, items, spells, runes }: Props) {
  const liveChampionIds = useMemo(() => uniqueChampionIds(liveGame.participants), [liveGame.participants]);
  const patchBucket = useMemo(() => patchBucketFromVersion(champions?.version), [champions?.version]);
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
  const blueChampionIds = useMemo(() => teamChampionIds(blueTeam), [blueTeam]);
  const redChampionIds = useMemo(() => teamChampionIds(redTeam), [redTeam]);
  const winConditionQuery = useQuery({
    queryKey: ['live-win-conditions', liveGame.gameQueueConfigId, patchBucket, blueChampionIds, redChampionIds],
    queryFn: () => getWinConditionAnalysis({
      blueChampionIds,
      redChampionIds,
      queueId: liveGame.gameQueueConfigId,
      patch: patchBucket,
      minGames: 5,
    }),
    enabled: blueChampionIds.length === 5 && redChampionIds.length === 5,
    staleTime: 60_000,
  });

  useEffect(() => {
    setManualOrder(false);
    setDraggedCard(null);
    setDragTarget(null);
  }, [liveGame.gameId]);

  useEffect(() => {
    if (manualOrder) return;
    setBlueTeam(initialBlue);
    setRedTeam(initialRed);
    setDraggedCard(null);
    setDragTarget(null);
  }, [initialBlue, initialRed, manualOrder]);

  const moveCardToIndex = (side: TeamSide, fromIndex: number, toIndex: number) => {
    setManualOrder(true);
    if (side === 'blue') {
      setBlueTeam((team) => moveParticipantToIndex(team, fromIndex, toIndex));
      return;
    }
    setRedTeam((team) => moveParticipantToIndex(team, fromIndex, toIndex));
  };

  const pairs = roles.map((role, index) => ({
    role,
    blue: blueTeam[index],
    red: redTeam[index],
  })).filter((pair) => pair.blue && pair.red);

  const statRequests = useMemo(() => {
    const requests: StatRequest[] = [];
    pairs.forEach((pair, index) => {
      if (!pair.blue || !pair.red) return;
      requests.push({
        key: statKey(index, 'blue'),
        filters: buildFilters(pair.blue, pair.red, pair.role),
      });
      requests.push({
        key: statKey(index, 'red'),
        filters: buildFilters(pair.red, pair.blue, pair.role),
      });
    });
    return requests;
  }, [pairs]);

  const statQueries = useQueries({
    queries: statRequests.map((request) => ({
      queryKey: ['live-board-item-slots', request.filters],
      queryFn: () => getItemSlots(request.filters),
      staleTime: 30_000,
    })),
  });

  const statsByKey = new Map<string, { loading: boolean; itemSlots: AnalyticsItemSlot[] }>();
  statRequests.forEach((request, index) => {
    const query = statQueries[index];
    statsByKey.set(request.key, {
      loading: query?.isLoading ?? false,
      itemSlots: query?.data?.results ?? [],
    });
  });

  return (
    <div className="game-board">
      <div className="match-type-container">
        <span>{queueLabel(liveGame.gameQueueConfigId)}</span>
        <span>{liveGame.platform}</span>
        <span>{liveGame.gameMode}</span>
      </div>
      <div className="cards-container">
        <div className="build-row blue-build-row">
          {pairs.map((pair, index) => {
            if (!pair.blue || !pair.red) return null;
            return (
              <MatchupBuildCard
                key={`blue-build-${participantKey(pair.blue, index)}-${participantKey(pair.red, index)}`}
                role={pair.role}
                side="blue"
                participant={pair.blue}
                opponent={pair.red}
                champions={champions}
                items={items}
                itemSlots={statsByKey.get(statKey(index, 'blue'))?.itemSlots ?? []}
                loading={statsByKey.get(statKey(index, 'blue'))?.loading ?? false}
              />
            );
          })}
        </div>
        <div className="champion-row blue-row">
          {blueTeam.map((participant, index) => (
            <LiveChampionCard
              key={participantKey(participant, index)}
              participant={participant}
              index={index}
              side="blue"
              champions={champions}
              spells={spells}
              runes={runes}
              dragging={draggedCard?.side === 'blue' && draggedCard.index === index}
              dropTarget={dragTarget?.side === 'blue' && dragTarget.index === index}
              onDragStart={() => setDraggedCard({ side: 'blue', index })}
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
        {blueChampionIds.length === 5 && redChampionIds.length === 5 ? (
          <WinConditionPanel
            analysis={winConditionQuery.data}
            yourSide={livePlayerSide(liveGame)}
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
              side="red"
              champions={champions}
              spells={spells}
              runes={runes}
              dragging={draggedCard?.side === 'red' && draggedCard.index === index}
              dropTarget={dragTarget?.side === 'red' && dragTarget.index === index}
              onDragStart={() => setDraggedCard({ side: 'red', index })}
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
        <div className="build-row red-build-row">
          {pairs.map((pair, index) => {
            if (!pair.blue || !pair.red) return null;
            return (
              <MatchupBuildCard
                key={`red-build-${participantKey(pair.red, index)}-${participantKey(pair.blue, index)}`}
                role={pair.role}
                side="red"
                participant={pair.red}
                opponent={pair.blue}
                champions={champions}
                items={items}
                itemSlots={statsByKey.get(statKey(index, 'red'))?.itemSlots ?? []}
                loading={statsByKey.get(statKey(index, 'red'))?.loading ?? false}
              />
            );
          })}
        </div>
      </div>
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
      />
      <WinConditionAlternatives
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
}: {
  title: string;
  side: TeamSide;
  team: WinConditionTeamProfile;
  metric?: WinConditionMetric;
}) {
  const condition = metric?.condition ?? team.primaryCondition;
  const rating = metric?.rating ?? team.primaryRating;
  return (
    <div className={`legacy-win-card ${side}`}>
      <h2>{title}</h2>
      <div className="legacy-win-images">
        <img className="legacy-condition-icon" src={conditionIconUrl(condition)} alt="" />
        <img className="legacy-rating-icon" src={ratingImageUrl(rating)} alt={rating} />
      </div>
      <div className="legacy-win-stat">
        <span>{team.sharpnessLabel ?? sharpnessFallback(team.primaryMargin)}</span>
        {metric && metric.games > 0 ? (
          <>
            <strong>Win Rate: {metric.winRate.toFixed(2)}%</strong>
            <span>Plan: {metric.planLabel ?? planLabelFallback(metric)}</span>
            <span>Total Games: {metric.games}</span>
            <span>Evidence: {metric.evidence?.level ?? evidenceLevelFallback(metric.games)}</span>
          </>
        ) : (
          <>
            <strong>Win Rate: --</strong>
            <span>Total Games: 0</span>
            <span>Evidence: No sample</span>
          </>
        )}
      </div>
    </div>
  );
}

function WinConditionAlternatives({
  metrics,
  selectedCondition,
  opponentCondition,
  onSelect,
}: {
  metrics: WinConditionMetric[];
  selectedCondition: string;
  opponentCondition: string;
  onSelect: (condition: string) => void;
}) {
  const alternatives = uniqueConditionMetrics(sortedAlternativeMetrics(metrics).filter((metric) => (
    metric.opponentCondition === opponentCondition
    && metric.condition !== selectedCondition
    && isPlayerFacingPlan(metric)
  )));
  return (
    <div className="legacy-stats-section alternatives-section">
      <h2>Alternatives</h2>
      <div className="alternative-list">
        {alternatives.length > 0 ? (
          alternatives.map((metric) => (
            <button className="alternative-item" key={`${metric.condition}-${metric.opponentCondition}`} type="button" onClick={() => onSelect(metric.condition)}>
              <span className="alternative-images">
                <img src={conditionIconUrl(metric.condition)} alt="" />
                <img src={ratingImageUrl(metric.rating)} alt={metric.rating} />
              </span>
              <span className="alternative-text">
                <strong>{metric.games > 0 ? metric.winRate.toFixed(2) : '--'}%</strong>
                <em>{metric.planLabel ?? planLabelFallback(metric)}</em>
                <em>{metric.games} Matches</em>
                <em>{metric.evidence?.level ?? evidenceLevelFallback(metric.games)}</em>
              </span>
            </button>
          ))
        ) : (
          <div className="alternative-empty">No other strong plans</div>
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
            <strong>{script.headline}</strong>
            <p>{script.playerRead}</p>
            <p>{planPairRead(metric)}</p>
            <p>{script.matchup}</p>
            <p>{script.modeRead}</p>
            <p>{script.timingRead}</p>
            {script.cautionRead ? <p className="match-read-caution">{script.cautionRead}</p> : null}
            {metric.evidence?.summary ? <p>{metric.evidence.summary}</p> : null}
            <em>{script.sampleRead}</em>
          </>
        ) : (
          <p>Select a win condition pairing to see the match read.</p>
        )}
      </div>
    </div>
  );
}

function WinConditionLengthChart({ metric }: { metric?: WinConditionMetric }) {
  const buckets = metric?.buckets ?? [];
  const points = buckets.map((bucket, index) => {
    const x = buckets.length <= 1 ? 50 : 8 + (index * 84) / (buckets.length - 1);
    const y = 56 - (Math.max(0, Math.min(100, bucket.winRate)) / 100) * 46;
    return { x, y, bucket };
  });
  const pointString = points.map((point) => `${point.x},${point.y}`).join(' ');
  return (
    <div className="legacy-stats-section chart-section">
      <h2>Winrate By Game Length</h2>
      <div className="chart-shell">
        <svg className="winrate-chart" viewBox="0 0 100 64" role="img" aria-label="Winrate by game length">
          <line x1="6" y1="10" x2="96" y2="10" />
          <line x1="6" y1="33" x2="96" y2="33" />
          <line x1="6" y1="56" x2="96" y2="56" />
          {pointString ? <polyline points={pointString} /> : null}
          {points.map((point) => (
            <circle key={point.bucket.bucket} cx={point.x} cy={point.y} r="2.2" />
          ))}
        </svg>
        <div className="chart-labels">
          {buckets.map((bucket) => (
            <span className={bucket.meetsMinGames ? '' : 'thin-sample'} key={bucket.bucket}>
              <b>{bucket.bucket}</b>
              {bucket.games > 0 ? `${bucket.winRate.toFixed(0)}%` : '--'}
            </span>
          ))}
        </div>
      </div>
    </div>
  );
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
  const otherMetrics = uniqueConditionMetrics(sortedAlternativeMetrics(metrics).filter((candidate) => (
    candidate.opponentCondition === opponentCondition
    && candidate.condition !== condition
    && isPlayerFacingPlan(candidate)
  )));
  return (
    <div className={`legacy-win-card enemy ${side}`}>
      <h2>Enemy Team's Win Condition</h2>
      <div className="legacy-win-images">
        <img className="legacy-condition-icon" src={conditionIconUrl(condition)} alt="" />
        <img className="legacy-rating-icon" src={ratingImageUrl(rating)} alt={rating} />
      </div>
      <div className="enemy-other-conditions">
        {otherMetrics.map((candidate) => (
          <button key={candidate.condition} type="button" onClick={() => onSelect(candidate.condition)} aria-label={`Show enemy ${candidate.condition}`}>
            <img src={conditionIconUrl(candidate.condition)} alt="" />
          </button>
        ))}
      </div>
    </div>
  );
}

function LiveChampionCard({
  participant,
  index,
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
}: {
  participant: LiveParticipant;
  index: number;
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
}) {
  const champion = championByKey(champions, participant.championId);
  const championName = champion?.name ?? String(participant.championId);
  const championUrl = championImageUrl(champions, participant.championId);
  const keystoneId = participant.perks?.perkIds?.[0];
  const secondaryStyleId = participant.perks?.perkSubStyle;
  const playerName = participant.riotId || participant.summonerName || 'Unknown player';

  return (
    <article
      className={`player-card ${side}-style${dragging ? ' dragging' : ''}${dropTarget ? ' drop-target' : ''}`}
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
      <div className="card-topline">
        {championUrl ? <img className="profile-picture" src={championUrl} alt={championName} /> : <div className="profile-picture profile-fallback">{participant.championId}</div>}
        <div className="player-title">
          <div className="summoner-name">{playerName}</div>
          <div className="champion-name">{championName}</div>
        </div>
      </div>
      <div className="player-stat-columns">
        <RankRecord rank={participant.rank} />
        <ChampionRecordBlock stats={participant.championStats} />
      </div>
      <div className="card-loadout">
        <div className="summoner-spells">
          {[participant.spell1Id, participant.spell2Id].map((spellId) => {
            const imageUrl = summonerSpellImageUrl(spells, spellId);
            const name = summonerSpellName(spells, spellId);
            return imageUrl ? <img key={spellId} src={imageUrl} alt={name} title={name} /> : <span className="spell-pill" key={spellId}>{spellId}</span>;
          })}
        </div>
        <div className="runes">
          <RuneIcon runes={runes} runeId={keystoneId} />
          <RuneStyleIcon runes={runes} styleId={secondaryStyleId} />
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
        <img className="ranked-icon" src={rankIconUrl()} alt="Rank unavailable" />
        <div className="tier-and-rank">Rank unavailable</div>
        <div className="winrate">Winrate: --</div>
        <div className="games-played">Games: --</div>
      </div>
    );
  }
  return (
    <div className={`rank-info${rank.rankAvailable === false ? ' rank-unranked' : ''}`}>
      <div className="stat-block-title">Ranked</div>
      <img className="ranked-icon" src={rankIconUrl(rank)} alt={rankLabel(rank)} />
      <div className="tier-and-rank">{rankLabel(rank)}</div>
      <div className="winrate">Winrate: {rank.winRate.toFixed(1)}%</div>
      <div className="games-played">Games: {rank.totalGames}</div>
    </div>
  );
}

function ChampionRecordBlock({ stats }: { stats?: ChampionRecord }) {
  if (!stats) {
    return (
      <div className="champion-performance stats-missing">
        <div className="stat-block-title">Champion</div>
        <div className="champion-stat-main">No samples</div>
        <div className="champion-stat-line">Champ WR: --</div>
        <div className="champion-stat-line">KDA: --</div>
        <div className="champion-stat-line">Avg: -- / -- / --</div>
      </div>
    );
  }
  return (
    <div className="champion-performance">
      <div className="stat-block-title">Champion</div>
      <div className="champion-stat-main">{stats.games} games</div>
      <div className="champion-stat-line">Champ WR: {stats.winRate.toFixed(1)}%</div>
      <div className="champion-stat-line">KDA: {stats.kda.toFixed(2)}</div>
      <div className="champion-stat-line">Avg: {stats.avgKills.toFixed(1)} / {stats.avgDeaths.toFixed(1)} / {stats.avgAssists.toFixed(1)}</div>
    </div>
  );
}

function MatchupBuildCard({
  role,
  side,
  participant,
  opponent,
  champions,
  items,
  itemSlots,
  loading,
}: {
  role: string;
  side: TeamSide;
  participant: LiveParticipant;
  opponent: LiveParticipant;
  champions?: ChampionData;
  items?: ItemData;
  itemSlots: AnalyticsItemSlot[];
  loading: boolean;
}) {
  const champion = championByKey(champions, participant.championId);
  const opponentChampion = championByKey(champions, opponent.championId);

  return (
    <article className={`match-build-card ${side}`}>
      <div className="build-heading">
        <span>{roleLabels[role] ?? role}</span>
        <strong>{champion?.name ?? participant.championId} into {opponentChampion?.name ?? opponent.championId}</strong>
      </div>
      <BuildSide side={side} itemSlots={itemSlots} loading={loading} items={items} />
    </article>
  );
}

function BuildSide({
  side,
  itemSlots,
  loading,
  items,
}: {
  side: 'blue' | 'red';
  itemSlots: AnalyticsItemSlot[];
  loading: boolean;
  items?: ItemData;
}) {
  if (loading) {
    return <div className={`build-side ${side} muted`}>Loading...</div>;
  }
  if (!itemSlots.length) {
    return <div className={`build-side ${side} muted`}>No item samples</div>;
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
    <div className="item-slot-column">
      <span className="item-slot-number">{ordinal(row.itemSlot)}</span>
      {imageUrl ? <img src={imageUrl} alt={name} title={name} /> : <span className="item-pill">{row.itemId}</span>}
      <div className="item-slot-stats">
        <strong>{row.winRate.toFixed(1)}%</strong>
        <span>{row.games} games</span>
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

function buildFilters(participant: LiveParticipant, opponent: LiveParticipant, role: string): BuildFilters {
  return {
    championId: participant.championId,
    opponentChampionId: opponent.championId,
    itemContext: itemContextForParticipant(participant, role),
    minGames: 1,
    limit: 6,
  };
}

function itemContextForParticipant(participant: LiveParticipant, role: string): BuildFilters['itemContext'] {
  if (hasSmite(participant)) return 'JUNGLE';
  if (role === 'UTILITY') return 'SUPPORT';
  return undefined;
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

function sharpnessFallback(primaryMargin?: number) {
  if (primaryMargin === undefined) return 'Team identity';
  if (primaryMargin <= 0) return 'Tied identity';
  if (primaryMargin === 1) return 'Contested identity';
  if (primaryMargin <= 3) return 'Flexible identity';
  if (primaryMargin <= 6) return 'Clear identity';
  return 'Sharp identity';
}

function planPairRead(metric: WinConditionMetric) {
  const ownPlan = metric.planLabel ?? planLabelFallback(metric);
  const enemyPlan = metric.opponentPlanLabel ?? (metric.opponentPrimary ? 'Primary' : 'Alternative');
  return `Plan context: your ${metric.condition} is a ${ownPlan.toLowerCase()} into the enemy ${metric.opponentCondition} ${enemyPlan.toLowerCase()}.`;
}

function livePlayerSide(liveGame: LiveGame): TeamSide {
  const participant = liveGame.participants.find((candidate) => candidate.puuid && candidate.puuid === liveGame.puuid);
  return participant?.teamId === 200 ? 'red' : 'blue';
}

function statKey(index: number, side: 'blue' | 'red') {
  return `${index}-${side}`;
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
