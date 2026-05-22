import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import type { ChampionData, ItemData, LiveGame, RuneData, SummonerSpellData } from '../api/types';
import { getBuildAdvice, getChampionRoleRates, getWinConditionAnalysis } from '../api/client';
import { FocusedBuildPanel, buildAdviceFilters, focusedBuildSelection } from './live-match/FocusedBuildPanel';
import { LiveMatchCardGrid } from './live-match/LiveMatchCardGrid';
import { LiveModeContext } from './live-match/LiveModeContext';
import { LiveModeRail } from './live-match/LiveModeRail';
import { MatchHeader } from './live-match/MatchHeader';
import {
  buildRoleRateMap,
  livePlayerSide,
  moveParticipantToIndex,
  orderTeam,
  patchBucketFromVersion,
  teamChampionIds,
  uniqueChampionIds,
} from './live-match/teamOrder';
import {
  type DraggedCard,
  type LiveMode,
  type TeamSide,
} from './live-match/types';
import { idsMatch } from './live-match/utils';

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
    focusedBuild ? buildAdviceFilters(focusedBuild.participant, focusedBuild.opponent, focusedBuild.role, patchBucket) : undefined
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

  const focusedBuildAdviceQuery = useQuery({
    queryKey: ['live-focused-build-advice', focusedBuildFilters],
    queryFn: () => getBuildAdvice(focusedBuildFilters!),
    enabled: showBuildMode && Boolean(focusedBuildFilters),
    staleTime: 30_000,
  });

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
          <LiveModeContext
            mode={liveMode}
            selection={focusedBuild}
            searchedParticipant={searchedParticipant}
            buildLoading={focusedBuildAdviceQuery.isFetching}
            winLoading={winConditionQuery.isFetching}
            winReady={blueChampionIds.length === 5 && redChampionIds.length === 5}
          />
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
              spells={spells}
              runes={runes}
              buildAdvice={focusedBuildAdviceQuery.data}
              loading={focusedBuildAdviceQuery.isLoading}
              selectedParticipantKey={selectedBuildParticipantKey}
              onSelectParticipant={(key) => {
                setSelectedBuildParticipantKey(key);
                setSelectedBuildOpponentKey('');
              }}
              selectedOpponentKey={selectedBuildOpponentKey}
              onSelectOpponent={setSelectedBuildOpponentKey}
            />
          ) : (
            <LiveMatchCardGrid
              blueTeam={blueTeam}
              redTeam={redTeam}
              selectedLaneIndex={selectedLaneIndex}
              onSelectLane={setSelectedLaneIndex}
              draggedCard={draggedCard}
              dragTarget={dragTarget}
              onDragStart={setDraggedCard}
              onDragTarget={setDragTarget}
              onMoveCard={moveCardToIndex}
              onClearDrag={() => {
                setDraggedCard(null);
                setDragTarget(null);
              }}
              showWinConditions={showWinConditionMode && blueChampionIds.length === 5 && redChampionIds.length === 5}
              winConditionAnalysis={winConditionQuery.data}
              winConditionLoading={winConditionQuery.isLoading}
              winConditionError={winConditionQuery.error instanceof Error ? winConditionQuery.error.message : undefined}
              yourSide={yourSide}
              champions={champions}
              spells={spells}
              runes={runes}
            />
          )}
        </div>
      </div>
    </div>
  );
}
