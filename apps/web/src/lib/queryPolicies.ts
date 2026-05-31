import type { DefaultOptions } from '@tanstack/react-query';
import { ApiError } from '../api/client';

export const queryStaleTime = {
  static: Infinity,
  patchList: 2 * 60_000,
  championGuide: 10 * 60_000,
  championIndex: 5 * 60_000,
  championRoleRates: 30 * 60_000,
  leaderboard: 2 * 60_000,
  summonerAlias: 5 * 60_000,
  summonerLive: 15_000,
  summonerProfile: 5 * 60_000,
  liveBuildAdvice: 2 * 60_000,
  liveWinConditions: 2 * 60_000,
} as const;

export const queryGcTime = {
  short: 5 * 60_000,
  medium: 30 * 60_000,
  long: 2 * 60 * 60_000,
  static: 24 * 60 * 60_000,
} as const;

export const defaultQueryOptions: DefaultOptions = {
  queries: {
    gcTime: queryGcTime.medium,
    refetchOnWindowFocus: false,
    retry: (failureCount, error) => {
      if (error instanceof ApiError && error.status >= 400 && error.status < 500 && error.status !== 429) {
        return false;
      }
      return failureCount < 2;
    },
  },
};
