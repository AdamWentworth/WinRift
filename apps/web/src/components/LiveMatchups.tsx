import { useQuery } from '@tanstack/react-query';
import { GripVertical } from 'lucide-react';
import { useEffect, useMemo, useState, type DragEvent } from 'react';
import type { ChampionData, ChampionRecord, ChampionRoleRate, ItemData, LiveGame, LiveParticipant, RankedRecord, RuneData, SummonerSpellData } from '../api/types';
import { getChampionRoleRates, getItemSlotsBatch, getWinConditionAnalysis } from '../api/client';
import {
  championByKey,
  championImageUrl,
  championSplashUrl,
  rankIconUrl,
  rankLabel,
  runeImageUrl,
  runeName,
  runeStyleName,
  summonerSpellImageUrl,
  summonerSpellName,
} from '../lib/staticData';
import { RoleIcon, roleLabel } from '../lib/roles';
import { FocusedBuildPanel, buildFilters, championBuildFiltersFor, focusedBuildSelection } from './live-match/FocusedBuildPanel';
import { LaneHeader, LaneTabs } from './live-match/LaneNavigation';
import { LiveModeRail } from './live-match/LiveModeRail';
import { MatchHeader } from './live-match/MatchHeader';
import { WinConditionPanel } from './live-match/WinConditionPanel';
import {
  roles,
  type DraggedCard,
  type LiveMode,
  type RoleRateMap,
  type TeamSide,
} from './live-match/types';
import { hasSmite, idsMatch, participantKey } from './live-match/utils';

type Props = {
  liveGame: LiveGame;
  champions?: ChampionData;
  items?: ItemData;
  spells?: SummonerSpellData;
  runes?: RuneData;
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
          <div className="player-role-chip"><RoleIcon role={role} /><span>{roleLabel(role)}</span></div>
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

function livePlayerSide(liveGame: LiveGame): TeamSide {
  const participant = liveGame.participants.find((candidate) => idsMatch(candidate.puuid, liveGame.puuid));
  return participant?.teamId === 200 ? 'red' : 'blue';
}
