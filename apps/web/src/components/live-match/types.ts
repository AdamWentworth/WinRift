import type { ChampionRoleRate, LiveParticipant } from '../../api/types';
import { ROLE_OPTIONS } from '../../lib/roles';

export const roles: string[] = ROLE_OPTIONS.map((role) => role.value);
export const BUILD_MATCHUP_MIN_GAMES = 5;
export const BUILD_BASELINE_MIN_GAMES = 10;
export const BUILD_SLOT_CANDIDATE_LIMIT = 12;

export type TeamSide = 'blue' | 'red';
export type LiveMode = 'match' | 'builds' | 'winConditions';

export type DraggedCard = {
  side: TeamSide;
  index: number;
};

export type RoleRateMap = Map<number, Map<string, ChampionRoleRate>>;

export type BuildParticipantOption = {
  key: string;
  side: TeamSide;
  role: string;
  index: number;
  participant: LiveParticipant;
};

export type FocusedBuildSelection = {
  side: TeamSide;
  role: string;
  participantKey: string;
  participant: LiveParticipant;
  opponentKey: string;
  opponent: LiveParticipant;
  participantOptions: BuildParticipantOption[];
  opponentOptions: BuildParticipantOption[];
};

export type BuildPathSummary = {
  weightedWinRate: number;
  totalGames: number;
};
