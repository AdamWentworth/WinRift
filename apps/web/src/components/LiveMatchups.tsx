import { useQueries } from '@tanstack/react-query';
import { useEffect, useLayoutEffect, useMemo, useRef, useState, type DragEvent } from 'react';
import { createPortal } from 'react-dom';
import type { AnalyticsItemSlot, BuildFilters, ChampionData, ChampionRecord, ItemData, LiveGame, LiveParticipant, RankedRecord, RuneData, SummonerSpellData } from '../api/types';
import { getItemSlots } from '../api/client';
import {
  championByKey,
  championImageUrl,
  itemImageUrl,
  itemName,
  rankIconUrl,
  rankLabel,
  runeImageUrl,
  runeName,
  runeStyleById,
  runeStyleImageUrl,
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

export function LiveMatchups({ liveGame, champions, items, spells, runes }: Props) {
  const initialBlue = useMemo(() => orderTeam(liveGame.participants.filter((participant) => participant.teamId === 100)), [liveGame.participants]);
  const initialRed = useMemo(() => orderTeam(liveGame.participants.filter((participant) => participant.teamId === 200)), [liveGame.participants]);
  const [blueTeam, setBlueTeam] = useState(initialBlue);
  const [redTeam, setRedTeam] = useState(initialRed);
  const [draggedCard, setDraggedCard] = useState<DraggedCard | null>(null);
  const [dragTarget, setDragTarget] = useState<DraggedCard | null>(null);

  useEffect(() => {
    setBlueTeam(initialBlue);
    setRedTeam(initialRed);
    setDraggedCard(null);
    setDragTarget(null);
  }, [initialBlue, initialRed, liveGame.gameId]);

  const moveCardToIndex = (side: TeamSide, fromIndex: number, toIndex: number) => {
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
        <RuneLoadout runes={runes} participant={participant} />
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

function RuneLoadout({ runes, participant }: { runes?: RuneData; participant: LiveParticipant }) {
  const triggerRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [position, setPosition] = useState({ left: 0, top: 0, align: 'center' as 'left' | 'center' | 'right' });
  const keystoneId = participant.perks?.perkIds?.[0];
  const secondaryStyleId = participant.perks?.perkSubStyle;

  useLayoutEffect(() => {
    if (!open || !triggerRef.current) return undefined;
    const updatePosition = () => {
      const rect = triggerRef.current?.getBoundingClientRect();
      if (!rect) return;
      const width = Math.min(340, window.innerWidth - 24);
      const height = Math.min(360, window.innerHeight - 24);
      const centeredLeft = rect.left + rect.width / 2 - width / 2;
      const left = Math.max(12, Math.min(centeredLeft, window.innerWidth - width - 12));
      const belowTop = rect.bottom + 10;
      const aboveTop = rect.top - height - 10;
      const top = belowTop + height <= window.innerHeight - 12 ? belowTop : Math.max(12, aboveTop);
      let align: 'left' | 'center' | 'right' = 'center';
      if (left <= 12) align = 'left';
      if (left >= window.innerWidth - width - 12) align = 'right';
      setPosition({ left, top, align });
    };
    updatePosition();
    window.addEventListener('resize', updatePosition);
    window.addEventListener('scroll', updatePosition, true);
    return () => {
      window.removeEventListener('resize', updatePosition);
      window.removeEventListener('scroll', updatePosition, true);
    };
  }, [open]);

  return (
    <div
      ref={triggerRef}
      className="runes rune-popover-trigger"
      tabIndex={0}
      onMouseEnter={() => setOpen(true)}
      onMouseLeave={() => setOpen(false)}
      onFocus={() => setOpen(true)}
      onBlur={() => setOpen(false)}
      aria-label="Rune selections"
    >
      <RuneIcon runes={runes} runeId={keystoneId} />
      <RuneStyleIcon runes={runes} styleId={secondaryStyleId} />
      {open ? <RunePopover runes={runes} participant={participant} position={position} /> : null}
    </div>
  );
}

function RunePopover({
  runes,
  participant,
  position,
}: {
  runes?: RuneData;
  participant: LiveParticipant;
  position: { left: number; top: number; align: 'left' | 'center' | 'right' };
}) {
  const perkIds = participant.perks?.perkIds ?? [];
  const primaryStyleId = participant.perks?.perkStyle;
  const secondaryStyleId = participant.perks?.perkSubStyle;
  const primaryStyle = runeStyleById(runes, primaryStyleId);
  const secondaryStyle = runeStyleById(runes, secondaryStyleId);
  const primarySelections = selectedRunesForStyle(runes, primaryStyleId, perkIds);
  const secondarySelections = selectedRunesForStyle(runes, secondaryStyleId, perkIds);

  return createPortal(
    <div
      className={`rune-popover rune-popover-${position.align}`}
      style={{
        left: `${position.left}px`,
        top: `${position.top}px`,
        width: `min(340px, calc(100vw - 24px))`,
      }}
    >
      <RuneTreeBlock
        runes={runes}
        title={primaryStyle?.name ?? runeStyleName(runes, primaryStyleId)}
        styleId={primaryStyleId}
        selections={primarySelections}
        fallbackIds={perkIds.slice(0, 4)}
      />
      <RuneTreeBlock
        runes={runes}
        title={secondaryStyle?.name ?? runeStyleName(runes, secondaryStyleId)}
        styleId={secondaryStyleId}
        selections={secondarySelections}
        fallbackIds={perkIds.slice(4, 6)}
      />
    </div>,
    document.body,
  );
}

function RuneTreeBlock({
  runes,
  title,
  styleId,
  selections,
  fallbackIds,
}: {
  runes?: RuneData;
  title: string;
  styleId?: number;
  selections: RuneSelection[];
  fallbackIds: number[];
}) {
  const visibleSelections = selections.length ? selections : fallbackIds.map((runeId, index) => ({ runeId, slotIndex: index, runeName: runeName(runes, runeId) }));
  const styleImageUrl = runeStyleImageUrl(runes, styleId);
  return (
    <section className="rune-tree-block">
      <div className="rune-tree-heading">
        {styleImageUrl ? <img src={styleImageUrl} alt="" /> : <span className="rune-tree-style-fallback">{styleId ?? '?'}</span>}
        <span>{title}</span>
      </div>
      <div className="rune-tree-list">
        {visibleSelections.length ? visibleSelections.map((selection) => (
          <div className="rune-tree-row" key={`${selection.slotIndex}-${selection.runeId}`}>
            <span className="rune-tree-slot">{selection.slotIndex + 1}</span>
            <RuneIcon runes={runes} runeId={selection.runeId} />
            <span className="rune-tree-name">{selection.runeName}</span>
          </div>
        )) : <div className="rune-tree-empty">No rune data</div>}
      </div>
    </section>
  );
}

type RuneSelection = {
  runeId: number;
  slotIndex: number;
  runeName: string;
};

function selectedRunesForStyle(runes: RuneData | undefined, styleId: number | undefined, perkIds: number[]): RuneSelection[] {
  const style = runeStyleById(runes, styleId);
  if (!style) return [];
  const selected = new Set(perkIds);
  return style.slots.flatMap((slot, slotIndex) => {
    const rune = slot.runes.find((candidate) => selected.has(candidate.id));
    return rune ? [{ runeId: rune.id, slotIndex, runeName: rune.name }] : [];
  });
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

function orderTeam(team: LiveParticipant[]) {
  if (team.length !== 5) return team;
  const smiteIndex = team.findIndex((participant) => participant.spell1Id === 11 || participant.spell2Id === 11);
  if (smiteIndex <= -1 || smiteIndex === 1) return team;
  const next = [...team];
  const [jungler] = next.splice(smiteIndex, 1);
  next.splice(1, 0, jungler);
  return next;
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
