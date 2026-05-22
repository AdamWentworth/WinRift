import type { ChampionRoleRate, LiveGame, LiveParticipant } from '../../api/types';
import { roles, type RoleRateMap, type TeamSide } from './types';
import { hasSmite, idsMatch } from './utils';

export function moveParticipantToIndex(team: LiveParticipant[], fromIndex: number, toIndex: number) {
  if (fromIndex === toIndex || fromIndex < 0 || toIndex < 0 || fromIndex >= team.length || toIndex >= team.length) {
    return team;
  }
  const next = [...team];
  const [current] = next.splice(fromIndex, 1);
  next.splice(toIndex, 0, current);
  return next;
}

export function orderTeam(team: LiveParticipant[], roleRates: RoleRateMap = new Map()) {
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

export function uniqueChampionIds(participants: LiveParticipant[]) {
  return [...new Set(participants.map((participant) => participant.championId).filter(Boolean))].sort((a, b) => a - b);
}

export function teamChampionIds(participants: LiveParticipant[]) {
  return participants.map((participant) => participant.championId).filter(Boolean);
}

export function patchBucketFromVersion(version?: string) {
  const match = version?.match(/^(\d+\.\d+)/);
  return match?.[1];
}

export function buildRoleRateMap(rows: ChampionRoleRate[] = []): RoleRateMap {
  const map: RoleRateMap = new Map();
  rows.forEach((row) => {
    const championRates = map.get(row.championId) ?? new Map<string, ChampionRoleRate>();
    championRates.set(row.role, row);
    map.set(row.championId, championRates);
  });
  return map;
}

export function livePlayerSide(liveGame: LiveGame): TeamSide {
  const participant = liveGame.participants.find((candidate) => idsMatch(candidate.puuid, liveGame.puuid));
  return participant?.teamId === 200 ? 'red' : 'blue';
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
