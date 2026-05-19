import { useQueries, useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useState, type DragEvent } from 'react';
import type { AnalyticsItemSlot, BuildFilters, ChampionData, ChampionRecord, ChampionRoleRate, ItemData, LiveGame, LiveParticipant, RankedRecord, RuneData, SummonerSpellData } from '../api/types';
import { getChampionRoleRates, getItemSlots } from '../api/client';
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
        filters: buildFilters(pair.blue.championId, pair.red.championId, pair.role),
      });
      requests.push({
        key: statKey(index, 'red'),
        filters: buildFilters(pair.red.championId, pair.blue.championId, pair.role),
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
        <div className="stat-row">
          {pairs.map((pair, index) => {
            if (!pair.blue || !pair.red) return null;
            return (
              <MatchupStatCard
                key={`${participantKey(pair.blue, index)}-${participantKey(pair.red, index)}`}
                role={pair.role}
                blue={pair.blue}
                red={pair.red}
                champions={champions}
                items={items}
                blueItemSlots={statsByKey.get(statKey(index, 'blue'))?.itemSlots ?? []}
                redItemSlots={statsByKey.get(statKey(index, 'red'))?.itemSlots ?? []}
                blueLoading={statsByKey.get(statKey(index, 'blue'))?.loading ?? false}
                redLoading={statsByKey.get(statKey(index, 'red'))?.loading ?? false}
              />
            );
          })}
        </div>
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

function MatchupStatCard({
  role,
  blue,
  red,
  champions,
  items,
  blueItemSlots,
  redItemSlots,
  blueLoading,
  redLoading,
}: {
  role: string;
  blue: LiveParticipant;
  red: LiveParticipant;
  champions?: ChampionData;
  items?: ItemData;
  blueItemSlots: AnalyticsItemSlot[];
  redItemSlots: AnalyticsItemSlot[];
  blueLoading: boolean;
  redLoading: boolean;
}) {
  const blueChampion = championByKey(champions, blue.championId);
  const redChampion = championByKey(champions, red.championId);

  return (
    <article className="match-stat-card">
      <div className="stat-heading">
        <span>{roleLabels[role] ?? role}</span>
        <strong>{blueChampion?.name ?? blue.championId} vs {redChampion?.name ?? red.championId}</strong>
      </div>
      <div className="stat-sides">
        <BuildSide side="blue" itemSlots={blueItemSlots} loading={blueLoading} items={items} />
        <BuildSide side="red" itemSlots={redItemSlots} loading={redLoading} items={items} />
      </div>
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
    <div className="item-slot-line">
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
    <div className="item-slot-line missing-item-slot">
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

function buildFilters(championId: number, opponentChampionId: number, role: string): BuildFilters {
  return {
    championId,
    opponentChampionId,
    role,
    minGames: 1,
    limit: 6,
  };
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

function buildRoleRateMap(rows: ChampionRoleRate[] = []): RoleRateMap {
  const map: RoleRateMap = new Map();
  rows.forEach((row) => {
    const championRates = map.get(row.championId) ?? new Map<string, ChampionRoleRate>();
    championRates.set(row.role, row);
    map.set(row.championId, championRates);
  });
  return map;
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
