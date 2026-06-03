import type { LiveParticipant } from '../../api/types';

export function participantDisplayName(participant: LiveParticipant) {
  return participant.riotId || participant.summonerName || 'Unknown player';
}
