import { useQueries } from '@tanstack/react-query';
import { useEffect, useLayoutEffect, useMemo, useRef, useState, type DragEvent } from 'react';
import { createPortal } from 'react-dom';
import type { AnalyticsItemSlot, BuildFilters, ChampionData, ChampionRecord, ItemData, LiveGame, LiveParticipant, RankedRecord, RuneData, RuneStyle, SummonerSpellData } from '../api/types';
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
  const closeTimerRef = useRef<number | undefined>(undefined);
  const [open, setOpen] = useState(false);
  const [position, setPosition] = useState({ left: 0, top: 0, align: 'center' as 'left' | 'center' | 'right' });
  const perkIds = selectedRuneIds(participant);
  const primaryStyleId = primaryStyleForParticipant(runes, participant);
  const secondaryStyleId = secondaryStyleForParticipant(runes, participant, primaryStyleId);
  const keystoneId = keystoneForParticipant(runes, participant, primaryStyleId);

  useEffect(() => () => {
    if (closeTimerRef.current) {
      window.clearTimeout(closeTimerRef.current);
    }
  }, []);

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

  const openPopover = () => {
    if (closeTimerRef.current) {
      window.clearTimeout(closeTimerRef.current);
      closeTimerRef.current = undefined;
    }
    setOpen(true);
  };
  const scheduleClose = () => {
    if (closeTimerRef.current) {
      window.clearTimeout(closeTimerRef.current);
    }
    closeTimerRef.current = window.setTimeout(() => setOpen(false), 120);
  };

  return (
    <div
      ref={triggerRef}
      className="runes rune-popover-trigger"
      tabIndex={0}
      onMouseEnter={openPopover}
      onMouseLeave={scheduleClose}
      onFocus={openPopover}
      onBlur={scheduleClose}
      aria-label="Rune selections"
    >
      <RuneIcon runes={runes} runeId={keystoneId} />
      <RuneStyleIcon runes={runes} styleId={secondaryStyleId} />
      {open ? <RunePopover runes={runes} participant={participant} position={position} onMouseEnter={openPopover} onMouseLeave={scheduleClose} /> : null}
    </div>
  );
}

function RunePopover({
  runes,
  participant,
  position,
  onMouseEnter,
  onMouseLeave,
}: {
  runes?: RuneData;
  participant: LiveParticipant;
  position: { left: number; top: number; align: 'left' | 'center' | 'right' };
  onMouseEnter: () => void;
  onMouseLeave: () => void;
}) {
  const perkIds = selectedRuneIds(participant);
  const primaryStyleId = primaryStyleForParticipant(runes, participant);
  const secondaryStyleId = secondaryStyleForParticipant(runes, participant, primaryStyleId);
  const primaryStyle = runeStyleById(runes, primaryStyleId);
  const secondaryStyle = runeStyleById(runes, secondaryStyleId);
  const primarySelections = selectedRunesForStyle(runes, primaryStyleId, perkIds);
  const secondarySelections = selectedRunesForStyle(runes, secondaryStyleId, perkIds);
  const statSelections = statSelectionsForParticipant(participant);
  const liveLimited = hasLimitedLiveRuneData(participant);

  return createPortal(
    <div
      className={`rune-popover rune-popover-${position.align}`}
      onMouseEnter={onMouseEnter}
      onMouseLeave={onMouseLeave}
      style={{
        left: `${position.left}px`,
        top: `${position.top}px`,
        width: `min(340px, calc(100vw - 24px))`,
      }}
    >
      {liveLimited ? <div className="rune-popover-note">Live data exposes keystone and rune trees. Full selections are available after match collection.</div> : null}
      <RuneTreeBlock
        runes={runes}
        title={primaryStyle?.name ?? runeStyleName(runes, primaryStyleId)}
        styleId={primaryStyleId}
        selections={primarySelections}
        slotIndexes={runeSlotIndexes(primaryStyle, 'primary')}
        emptyLabel={liveLimited ? 'Unavailable live' : 'Unknown selection'}
      />
      <RuneTreeBlock
        runes={runes}
        title={secondaryStyle?.name ?? runeStyleName(runes, secondaryStyleId)}
        styleId={secondaryStyleId}
        selections={secondarySelections}
        slotIndexes={runeSlotIndexes(secondaryStyle, 'secondary')}
        emptyLabel={liveLimited ? 'Unavailable live' : 'Not selected'}
      />
      <StatShardBlock selections={statSelections} emptyLabel={liveLimited ? 'Unavailable live' : 'Unknown'} />
    </div>,
    document.body,
  );
}

function RuneTreeBlock({
  runes,
  title,
  styleId,
  selections,
  slotIndexes,
  emptyLabel,
}: {
  runes?: RuneData;
  title: string;
  styleId?: number;
  selections: RuneSelection[];
  slotIndexes: number[];
  emptyLabel: string;
}) {
  const rows = runeRows(selections, slotIndexes);
  const styleImageUrl = runeStyleImageUrl(runes, styleId);
  return (
    <section className="rune-tree-block">
      <div className="rune-tree-heading">
        {styleImageUrl ? <img src={styleImageUrl} alt="" /> : <span className="rune-tree-style-fallback">{styleId ?? '?'}</span>}
        <span>{title}</span>
      </div>
      <div className="rune-tree-list">
        {rows.map((row) => (
          <div className={`rune-tree-row${row.selection ? '' : ' rune-tree-row-empty'}`} key={row.selection ? `${row.slotIndex}-${row.selection.runeId}` : `empty-${row.slotIndex}`}>
            <span className="rune-tree-slot">{slotLabel(row.slotIndex)}</span>
            {row.selection ? <RuneIcon runes={runes} runeId={row.selection.runeId} /> : <span className="rune-fallback">?</span>}
            <span className="rune-tree-name">{row.selection?.runeName ?? emptyLabel}</span>
          </div>
        ))}
      </div>
    </section>
  );
}

function StatShardBlock({ selections, emptyLabel }: { selections: StatShardSelection[]; emptyLabel: string }) {
  const rows = ['Offense', 'Flex', 'Defense'].map((label, index) => ({
    label,
    selection: selections[index],
  }));
  return (
    <section className="rune-tree-block">
      <div className="rune-tree-heading">
        <span className="rune-tree-style-fallback">+</span>
        <span>Stat Shards</span>
      </div>
      <div className="rune-tree-list stat-shard-list">
        {rows.map(({ label, selection }) => (
          <div className={`rune-tree-row${selection ? '' : ' rune-tree-row-empty'}`} key={label}>
            <span className="rune-tree-slot">{label.slice(0, 1)}</span>
            {selection ? <img src={selection.iconUrl} alt={selection.name} title={selection.name} /> : <span className="rune-fallback">?</span>}
            <span className="rune-tree-name">{selection ? `${label}: ${selection.name}` : `${label}: ${emptyLabel}`}</span>
          </div>
        ))}
      </div>
    </section>
  );
}

type RuneSelection = {
  runeId: number;
  slotIndex: number;
  runeName: string;
};

type StatShardSelection = {
  id: number;
  name: string;
  iconUrl: string;
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

function selectedRuneIds(participant: LiveParticipant) {
  const styleSelections = participant.perks?.styles?.flatMap((style) => style.selections?.map((selection) => selection.perk) ?? []) ?? [];
  if (styleSelections.length) return styleSelections;
  return (participant.perks?.perkIds ?? []).filter((perkId) => !isStatShardId(perkId));
}

function primaryStyleForParticipant(runes: RuneData | undefined, participant: LiveParticipant) {
  if (participant.perks?.perkStyle) return participant.perks.perkStyle;
  const explicitPrimary = participant.perks?.styles?.find((style) => style.description === 'primaryStyle')?.style;
  if (explicitPrimary) return explicitPrimary;
  return styleIdForRune(runes, selectedRuneIds(participant)[0]);
}

function secondaryStyleForParticipant(runes: RuneData | undefined, participant: LiveParticipant, primaryStyleId?: number) {
  if (participant.perks?.perkSubStyle) return participant.perks.perkSubStyle;
  const explicitSecondary = participant.perks?.styles?.find((style) => style.description === 'subStyle')?.style;
  if (explicitSecondary) return explicitSecondary;
  const runeIDs = selectedRuneIds(participant);
  for (const runeId of runeIDs) {
    const styleId = styleIdForRune(runes, runeId);
    if (styleId && styleId !== primaryStyleId) return styleId;
  }
  return undefined;
}

function keystoneForParticipant(runes: RuneData | undefined, participant: LiveParticipant, primaryStyleId?: number) {
  const primarySelections = selectedRunesForStyle(runes, primaryStyleId, selectedRuneIds(participant));
  return primarySelections[0]?.runeId ?? selectedRuneIds(participant)[0];
}

function styleIdForRune(runes: RuneData | undefined, runeId: number | undefined) {
  if (!runes || !runeId) return undefined;
  return runes.data.find((style) => style.slots.some((slot) => slot.runes.some((rune) => rune.id === runeId)))?.id;
}

function statSelectionsForParticipant(participant: LiveParticipant): StatShardSelection[] {
  const statPerks = participant.perks?.statPerks;
  const ids = statPerks
    ? [statPerks.offense, statPerks.flex, statPerks.defense]
    : (participant.perks?.perkIds ?? []).filter(isStatShardId).slice(-3);
  return ids
    .filter((id): id is number => Boolean(id))
    .map((id) => statShardSelection(id));
}

function statShardSelection(id: number): StatShardSelection {
  const metadata = statShardMetadata[id] ?? {
    name: `Stat Shard ${id}`,
    icon: 'StatModsAdaptiveForceIcon',
  };
  return {
    id,
    name: metadata.name,
    iconUrl: `https://ddragon.leagueoflegends.com/cdn/img/perk-images/StatMods/${metadata.icon}.png`,
  };
}

function runeSlotIndexes(style: RuneStyle | undefined, kind: 'primary' | 'secondary') {
  if (!style) return kind === 'primary' ? [0, 1, 2, 3] : [1, 2, 3];
  const indexes = style.slots.map((_, index) => index);
  if (kind === 'secondary' && indexes.length >= 4) return indexes.filter((index) => index > 0);
  return indexes;
}

function runeRows(selections: RuneSelection[], slotIndexes: number[]) {
  if (!slotIndexes.length) return selections.map((selection) => ({ slotIndex: selection.slotIndex, selection }));
  return slotIndexes.map((slotIndex) => ({
    slotIndex,
    selection: selections.find((selection) => selection.slotIndex === slotIndex),
  }));
}

function slotLabel(index: number) {
  return index === 0 ? 'K' : String(index);
}

function hasLimitedLiveRuneData(participant: LiveParticipant) {
  const runeIds = selectedRuneIds(participant);
  const hasMatchStyleSelections = Boolean(participant.perks?.styles?.some((style) => style.selections?.length));
  const hasStatPerks = Boolean(participant.perks?.statPerks) || (participant.perks?.perkIds ?? []).some(isStatShardId);
  return !hasMatchStyleSelections && runeIds.length <= 1 && !hasStatPerks;
}

function isStatShardId(perkId: number) {
  return perkId >= 5000 && perkId < 6000;
}

const statShardMetadata: Record<number, { name: string; icon: string }> = {
  5001: { name: 'Health Scaling', icon: 'StatModsHealthScalingIcon' },
  5002: { name: 'Armor', icon: 'StatModsArmorIcon' },
  5003: { name: 'Magic Resist', icon: 'StatModsMagicResIcon' },
  5005: { name: 'Attack Speed', icon: 'StatModsAttackSpeedIcon' },
  5007: { name: 'Ability Haste', icon: 'StatModsCDRScalingIcon' },
  5008: { name: 'Adaptive Force', icon: 'StatModsAdaptiveForceIcon' },
  5010: { name: 'Move Speed', icon: 'StatModsMovementSpeedIcon' },
  5011: { name: 'Health', icon: 'StatModsHealthPlusIcon' },
  5013: { name: 'Tenacity and Slow Resist', icon: 'StatModsTenacityIcon' },
};

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
