import type { ChampionData, LiveParticipant, RuneData, SummonerSpellData, WinConditionAnalysisResponse } from '../../api/types';
import { LaneHeader, LaneTabs } from './LaneNavigation';
import { LiveChampionCard } from './LiveChampionCard';
import { WinConditionPanel } from './WinConditionPanel';
import { roles, type DraggedCard, type TeamSide } from './types';
import { participantKey } from './utils';

export function LiveMatchCardGrid({
  blueTeam,
  redTeam,
  selectedLaneIndex,
  onSelectLane,
  draggedCard,
  dragTarget,
  onDragStart,
  onDragTarget,
  onMoveCard,
  onClearDrag,
  showWinConditions,
  winConditionAnalysis,
  winConditionLoading,
  winConditionError,
  yourSide,
  champions,
  spells,
  runes,
}: {
  blueTeam: LiveParticipant[];
  redTeam: LiveParticipant[];
  selectedLaneIndex: number;
  onSelectLane: (index: number) => void;
  draggedCard: DraggedCard | null;
  dragTarget: DraggedCard | null;
  onDragStart: (card: DraggedCard) => void;
  onDragTarget: (card: DraggedCard) => void;
  onMoveCard: (side: TeamSide, fromIndex: number, toIndex: number) => void;
  onClearDrag: () => void;
  showWinConditions: boolean;
  winConditionAnalysis?: WinConditionAnalysisResponse;
  winConditionLoading: boolean;
  winConditionError?: string;
  yourSide: TeamSide;
  champions?: ChampionData;
  spells?: SummonerSpellData;
  runes?: RuneData;
}) {
  return (
    <div className="cards-container">
      <LaneTabs selectedIndex={selectedLaneIndex} onSelect={onSelectLane} />
      <LaneHeader />
      <TeamRow
        side="blue"
        team={blueTeam}
        selectedLaneIndex={selectedLaneIndex}
        draggedCard={draggedCard}
        dragTarget={dragTarget}
        onDragStart={onDragStart}
        onDragTarget={onDragTarget}
        onMoveCard={onMoveCard}
        onClearDrag={onClearDrag}
        champions={champions}
        spells={spells}
        runes={runes}
      />
      {showWinConditions ? (
        <WinConditionPanel
          analysis={winConditionAnalysis}
          yourSide={yourSide}
          loading={winConditionLoading}
          error={winConditionError}
        />
      ) : null}
      <TeamRow
        side="red"
        team={redTeam}
        selectedLaneIndex={selectedLaneIndex}
        draggedCard={draggedCard}
        dragTarget={dragTarget}
        onDragStart={onDragStart}
        onDragTarget={onDragTarget}
        onMoveCard={onMoveCard}
        onClearDrag={onClearDrag}
        champions={champions}
        spells={spells}
        runes={runes}
      />
    </div>
  );
}

function TeamRow({
  side,
  team,
  selectedLaneIndex,
  draggedCard,
  dragTarget,
  onDragStart,
  onDragTarget,
  onMoveCard,
  onClearDrag,
  champions,
  spells,
  runes,
}: {
  side: TeamSide;
  team: LiveParticipant[];
  selectedLaneIndex: number;
  draggedCard: DraggedCard | null;
  dragTarget: DraggedCard | null;
  onDragStart: (card: DraggedCard) => void;
  onDragTarget: (card: DraggedCard) => void;
  onMoveCard: (side: TeamSide, fromIndex: number, toIndex: number) => void;
  onClearDrag: () => void;
  champions?: ChampionData;
  spells?: SummonerSpellData;
  runes?: RuneData;
}) {
  return (
    <div className={`champion-row ${side}-row`}>
      {team.map((participant, index) => (
        <LiveChampionCard
          key={participantKey(participant, index)}
          participant={participant}
          index={index}
          role={roles[index]}
          side={side}
          champions={champions}
          spells={spells}
          runes={runes}
          dragging={draggedCard?.side === side && draggedCard.index === index}
          dropTarget={dragTarget?.side === side && dragTarget.index === index}
          onDragStart={() => onDragStart({ side, index })}
          mobileActive={index === selectedLaneIndex}
          onDragOver={(event) => {
            event.preventDefault();
            onDragTarget({ side, index });
          }}
          onDrop={(event) => {
            event.preventDefault();
            if (draggedCard?.side === side) {
              onMoveCard(side, draggedCard.index, index);
            }
            onClearDrag();
          }}
          onDragEnd={onClearDrag}
        />
      ))}
    </div>
  );
}
