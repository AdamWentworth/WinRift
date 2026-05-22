import type { LiveParticipant } from '../../api/types';

export function hasSmite(participant: LiveParticipant) {
  return participant.spell1Id === 11 || participant.spell2Id === 11;
}

export function idsMatch(left?: string, right?: string) {
  const normalizedLeft = left?.trim();
  const normalizedRight = right?.trim();
  return Boolean(normalizedLeft && normalizedRight && normalizedLeft === normalizedRight);
}

export function sameParticipantIdentity(participant: LiveParticipant, target: LiveParticipant) {
  return idsMatch(participant.puuid, target.puuid) || idsMatch(participant.summonerId, target.summonerId);
}

export function participantKey(participant: LiveParticipant, index: number) {
  return `${participant.teamId}-${participant.summonerId ?? participant.riotId ?? participant.championId}-${index}`;
}

export function liveParticipantName(participant?: LiveParticipant) {
  return participant?.riotId || participant?.summonerName || 'Unknown player';
}
