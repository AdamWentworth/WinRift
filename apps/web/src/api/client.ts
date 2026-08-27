import type { AccountAliasResolution, AccountAliasSearchResponse, AnalyticsBuildResponse, AnalyticsItemSlotBatchRequest, AnalyticsItemSlotBatchResponse, AnalyticsItemSlotResponse, AnalyticsPatchesResponse, BuildAdviceResponse, BuildFilters, ChampionData, ChampionGuideIndexResponse, ChampionGuideResponse, ChampionPageBundleResponse, ChampionRoleRatesResponse, ChampionSplashData, ItemData, LiveGame, RuneData, SummonerLeaderboardResponse, SummonerProfile, SummonerSpellData, WinConditionAnalysisRequest, WinConditionAnalysisResponse } from './types';

// Production serves the frontend and API from one origin. Local development can
// still opt into a separate API host through VITE_API_URL.
const API_URL = (import.meta.env.VITE_API_URL ?? (import.meta.env.DEV ? 'http://localhost:8000' : '')).replace(/\/$/, '');

export type RequestOptions = {
  signal?: AbortSignal;
};

export class ApiError extends Error {
  status: number;
  path: string;

  constructor(message: string, status: number, path: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.path = path;
  }
}

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const response = await fetch(`${API_URL}${path}`, {
    headers: { Accept: 'application/json' },
    signal: options.signal,
  });
  if (!response.ok) {
    const detail = await response.json().catch(() => ({}));
    throw new ApiError(detail.detail ?? `Request failed with status ${response.status}`, response.status, path);
  }
  return response.json() as Promise<T>;
}

async function post<T>(path: string, body: unknown, options: RequestOptions = {}): Promise<T> {
  const response = await fetch(`${API_URL}${path}`, {
    method: 'POST',
    headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
    signal: options.signal,
  });
  if (!response.ok) {
    const detail = await response.json().catch(() => ({}));
    throw new ApiError(detail.detail ?? `Request failed with status ${response.status}`, response.status, path);
  }
  return response.json() as Promise<T>;
}

export function getChampions(options?: RequestOptions) {
  return request<ChampionData>('/api/static/champions', options);
}

export function getChampionSplashes(options?: RequestOptions) {
  return request<ChampionSplashData>('/api/static/champion-splashes', options);
}

export function getItems(options?: RequestOptions) {
  return request<ItemData>('/api/static/items', options);
}

export function getSummonerSpells(options?: RequestOptions) {
  return request<SummonerSpellData>('/api/static/summoner-spells', options);
}

export function getRunes(options?: RequestOptions) {
  return request<RuneData>('/api/static/runes', options);
}

export function getBuilds(filters: BuildFilters, options?: RequestOptions) {
  const params = new URLSearchParams();
  if (filters.championId) params.set('championId', String(filters.championId));
  if (filters.role) params.set('role', filters.role);
  if (filters.opponentChampionId) params.set('opponentChampionId', String(filters.opponentChampionId));
  if (filters.patch) params.set('patch', filters.patch);
  if (filters.rankBucket) params.set('rankBucket', filters.rankBucket);
  params.set('minGames', String(filters.minGames));
  if (filters.limit) params.set('limit', String(filters.limit));
  return request<AnalyticsBuildResponse>(`/api/analytics/builds?${params.toString()}`, options);
}

export function getBuildAdvice(filters: BuildFilters & { championMinGames?: number }, options?: RequestOptions) {
  const params = new URLSearchParams();
  if (filters.championId) params.set('championId', String(filters.championId));
  if (filters.role) params.set('role', filters.role);
  if (filters.itemContext) params.set('itemContext', filters.itemContext);
  if (filters.opponentChampionId) params.set('opponentChampionId', String(filters.opponentChampionId));
  if (filters.patch) params.set('patch', filters.patch);
  if (filters.rankBucket) params.set('rankBucket', filters.rankBucket);
  params.set('minGames', String(filters.minGames));
  if (filters.championMinGames) params.set('championMinGames', String(filters.championMinGames));
  if (filters.limit) params.set('limit', String(filters.limit));
  return request<BuildAdviceResponse>(`/api/analytics/build-advice?${params.toString()}`, options);
}

export function getChampionGuide(filters: BuildFilters, options?: RequestOptions) {
  const params = new URLSearchParams();
  if (filters.championId) params.set('championId', String(filters.championId));
  if (filters.role) params.set('role', filters.role);
  if (filters.patch) params.set('patch', filters.patch);
  if (filters.rankBucket) params.set('rankBucket', filters.rankBucket);
  params.set('minGames', String(filters.minGames));
  if (filters.limit) params.set('limit', String(filters.limit));
  return request<ChampionGuideResponse>(`/api/analytics/champion-guide?${params.toString()}`, options);
}

export function getChampionPageBundle(filters: BuildFilters & { championMinGames?: number; guideMinGames?: number; guideLimit?: number; indexMinGames?: number; indexLimit?: number; queueId?: number }, options?: RequestOptions) {
  const params = new URLSearchParams();
  if (filters.championId) params.set('championId', String(filters.championId));
  if (filters.role) params.set('role', filters.role);
  if (filters.itemContext) params.set('itemContext', filters.itemContext);
  if (filters.opponentChampionId) params.set('opponentChampionId', String(filters.opponentChampionId));
  if (filters.patch) params.set('patch', filters.patch);
  if (filters.rankBucket) params.set('rankBucket', filters.rankBucket);
  params.set('minGames', String(filters.minGames));
  if (filters.championMinGames) params.set('championMinGames', String(filters.championMinGames));
  if (filters.limit) params.set('limit', String(filters.limit));
  if (filters.guideMinGames) params.set('guideMinGames', String(filters.guideMinGames));
  if (filters.guideLimit) params.set('guideLimit', String(filters.guideLimit));
  if (filters.indexMinGames) params.set('indexMinGames', String(filters.indexMinGames));
  if (filters.indexLimit) params.set('indexLimit', String(filters.indexLimit));
  if (filters.queueId) params.set('queueId', String(filters.queueId));
  return request<ChampionPageBundleResponse>(`/api/analytics/champion-page?${params.toString()}`, options);
}

export function getChampionGuideIndex(filters: BuildFilters, options?: RequestOptions) {
  const params = new URLSearchParams();
  if (filters.role) params.set('role', filters.role);
  if (filters.patch) params.set('patch', filters.patch);
  if (filters.rankBucket) params.set('rankBucket', filters.rankBucket);
  params.set('minGames', String(filters.minGames ?? 1));
  if (filters.limit) params.set('limit', String(filters.limit));
  return request<ChampionGuideIndexResponse>(`/api/analytics/champion-guides?${params.toString()}`, options);
}

export function getItemSlots(filters: BuildFilters, options?: RequestOptions) {
  const params = new URLSearchParams();
  if (filters.championId) params.set('championId', String(filters.championId));
  if (filters.role) params.set('role', filters.role);
  if (filters.itemContext) params.set('itemContext', filters.itemContext);
  if (filters.opponentChampionId) params.set('opponentChampionId', String(filters.opponentChampionId));
  if (filters.patch) params.set('patch', filters.patch);
  if (filters.rankBucket) params.set('rankBucket', filters.rankBucket);
  params.set('minGames', String(filters.minGames));
  if (filters.limit) params.set('limit', String(filters.limit));
  if (filters.fallback) params.set('fallback', 'true');
  return request<AnalyticsItemSlotResponse>(`/api/analytics/item-slots?${params.toString()}`, options);
}

export function getItemSlotsBatch(requests: AnalyticsItemSlotBatchRequest[], options?: RequestOptions) {
  return post<AnalyticsItemSlotBatchResponse>('/api/analytics/item-slots/batch', { requests }, options);
}

export function getChampionRoleRates(championIds: number[], queueId: number, options?: RequestOptions) {
  const params = new URLSearchParams({
    championIds: championIds.join(','),
    queueId: String(queueId),
  });
  return request<ChampionRoleRatesResponse>(`/api/analytics/champion-roles?${params.toString()}`, options);
}

export function getAnalyticsPatches(queueId = 420, options?: RequestOptions) {
  const params = new URLSearchParams({ queueId: String(queueId) });
  return request<AnalyticsPatchesResponse>(`/api/analytics/patches?${params.toString()}`, options);
}

export function getWinConditionAnalysis(body: WinConditionAnalysisRequest, options?: RequestOptions) {
  return post<WinConditionAnalysisResponse>('/api/analytics/win-conditions', body, options);
}

export function resolveAccountAlias(gameName: string, platform: string, options?: RequestOptions) {
  const params = new URLSearchParams({ gameName, platform });
  return request<AccountAliasResolution>(`/api/account/alias?${params.toString()}`, options);
}

export function searchAccountAliases(gameName: string, platform: string, limit = 6, options?: RequestOptions) {
  const params = new URLSearchParams({ gameName, platform, limit: String(limit) });
  return request<AccountAliasSearchResponse>(`/api/account/aliases?${params.toString()}`, options);
}

export function getSummonerProfile(gameName: string, tagLine: string, platform: string, options?: RequestOptions) {
  const params = new URLSearchParams({ gameName, tagLine, platform });
  return request<SummonerProfile>(`/api/summoner/profile?${params.toString()}`, options);
}

export function getSummonerLeaderboard(platform: string, limit = 50, options?: RequestOptions) {
  const params = new URLSearchParams({ platform, limit: String(limit) });
  return request<SummonerLeaderboardResponse>(`/api/summoners/leaderboard?${params.toString()}`, options);
}

export function getLiveGame(gameName: string, tagLine: string, platform: string, options?: RequestOptions) {
  const params = new URLSearchParams({ gameName, tagLine, platform });
  return request<LiveGame>(`/api/live-game?${params.toString()}`, options);
}
