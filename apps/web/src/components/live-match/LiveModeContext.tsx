import type { LiveParticipant } from '../../api/types';
import { StatusChip } from '../ui/StatusChip';
import type { FocusedBuildSelection, LiveMode } from './types';

export function LiveModeContext({
  mode,
  selection,
  searchedParticipant,
  buildLoading,
  winLoading,
  winReady,
}: {
  mode: LiveMode;
  selection?: FocusedBuildSelection;
  searchedParticipant?: LiveParticipant;
  buildLoading: boolean;
  winLoading: boolean;
  winReady: boolean;
}) {
  if (mode === 'builds') {
    const player = selection ? liveParticipantName(selection.participant) : liveParticipantName(searchedParticipant);
    const opponent = selection ? liveParticipantName(selection.opponent) : 'lane opponent';
    return (
      <section className="live-mode-context builds">
        <div>
          <span>Builds Mode</span>
          <strong>{selection ? `${player} vs ${opponent}` : 'Focused item matchup'}</strong>
          <p>Build stats load only in this mode. Switch the player or opponent to compare exact matchup items against the champion-wide baseline.</p>
        </div>
        <StatusChip className="live-mode-status" tone={buildLoading ? 'early' : 'useful'}>{buildLoading ? 'Loading build stats' : 'Focused query active'}</StatusChip>
      </section>
    );
  }
  if (mode === 'winConditions') {
    return (
      <section className="live-mode-context win-conditions">
        <div>
          <span>Win Conditions Mode</span>
          <strong>Team strategy read</strong>
          <p>Composition metrics load only here. Reorder cards first if Riot's live lane order looks wrong, then read the primary strategy and timing windows.</p>
        </div>
        <StatusChip className="live-mode-status" tone={!winReady ? 'thin' : winLoading ? 'early' : 'useful'}>{!winReady ? 'Needs 5v5 order' : winLoading ? 'Loading strategy stats' : 'Strategy query active'}</StatusChip>
      </section>
    );
  }
  return (
    <section className="live-mode-context match">
      <div>
        <span>Match Overview</span>
        <strong>Scout the room first</strong>
        <p>All ten player cards stay lightweight here. Builds and win-condition analytics stay idle until you open those modes.</p>
      </div>
      <StatusChip className="live-mode-status" tone={searchedParticipant ? 'good' : 'thin'}>{searchedParticipant ? `Focused on ${liveParticipantName(searchedParticipant)}` : 'No searched player marker'}</StatusChip>
    </section>
  );
}

function liveParticipantName(participant?: LiveParticipant) {
  return participant?.riotId || participant?.summonerName || 'Unknown player';
}
