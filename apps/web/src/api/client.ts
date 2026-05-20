import type { AccountAliasResolution, AccountAliasSearchResponse, AnalyticsBuildResponse, AnalyticsItemSlotBatchRequest, AnalyticsItemSlotBatchResponse, AnalyticsItemSlotResponse, BuildFilters, ChampionData, ChampionRoleRatesResponse, ChampionSplashData, ItemData, LiveGame, RuneData, SummonerSpellData, WinConditionAnalysisRequest, WinConditionAnalysisResponse } from './types';

const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8000';

async function request<T>(path: string): Promise<T> {
  const response = await fetch(`${API_URL}${path}`);
  if (!response.ok) {
    const detail = await response.json().catch(() => ({}));
    throw new Error(detail.detail ?? `Request failed with status ${response.status}`);
  }
  return response.json() as Promise<T>;
}

async function post<T>(path: string, body: unknown): Promise<T> {
  const response = await fetch(`${API_URL}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!response.ok) {
    const detail = await response.json().catch(() => ({}));
    throw new Error(detail.detail ?? `Request failed with status ${response.status}`);
  }
  return response.json() as Promise<T>;
}

export function getChampions() {
  return request<ChampionData>('/api/static/champions');
}

export function getChampionSplashes() {
  return request<ChampionSplashData>('/api/static/champion-splashes');
}

export function getItems() {
  return request<ItemData>('/api/static/items');
}

export function getSummonerSpells() {
  return request<SummonerSpellData>('/api/static/summoner-spells');
}

export function getRunes() {
  return request<RuneData>('/api/static/runes');
}

export function getBuilds(filters: BuildFilters) {
  const params = new URLSearchParams();
  if (filters.championId) params.set('championId', String(filters.championId));
  if (filters.role) params.set('role', filters.role);
  if (filters.opponentChampionId) params.set('opponentChampionId', String(filters.opponentChampionId));
  if (filters.patch) params.set('patch', filters.patch);
  if (filters.rankBucket) params.set('rankBucket', filters.rankBucket);
  params.set('minGames', String(filters.minGames));
  if (filters.limit) params.set('limit', String(filters.limit));
  return request<AnalyticsBuildResponse>(`/api/analytics/builds?${params.toString()}`);
}

export function getItemSlots(filters: BuildFilters) {
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
  return request<AnalyticsItemSlotResponse>(`/api/analytics/item-slots?${params.toString()}`);
}

export function getItemSlotsBatch(requests: AnalyticsItemSlotBatchRequest[]) {
  return post<AnalyticsItemSlotBatchResponse>('/api/analytics/item-slots/batch', { requests });
}

export function getChampionRoleRates(championIds: number[], queueId: number) {
  const params = new URLSearchParams({
    championIds: championIds.join(','),
    queueId: String(queueId),
  });
  return request<ChampionRoleRatesResponse>(`/api/analytics/champion-roles?${params.toString()}`);
}

export function getWinConditionAnalysis(body: WinConditionAnalysisRequest) {
  return post<WinConditionAnalysisResponse>('/api/analytics/win-conditions', body);
}

export function resolveAccountAlias(gameName: string, platform: string) {
  const params = new URLSearchParams({ gameName, platform });
  return request<AccountAliasResolution>(`/api/account/alias?${params.toString()}`);
}

export function searchAccountAliases(gameName: string, platform: string, limit = 6) {
  const params = new URLSearchParams({ gameName, platform, limit: String(limit) });
  return request<AccountAliasSearchResponse>(`/api/account/aliases?${params.toString()}`);
}

export function getLiveGame(gameName: string, tagLine: string, platform: string) {
  const params = new URLSearchParams({ gameName, tagLine, platform });
  return request<LiveGame>(`/api/live-game?${params.toString()}`);
}
