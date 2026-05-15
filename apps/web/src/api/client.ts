import type { AnalyticsBuildResponse, BuildFilters, ChampionData, ItemData, LiveGame } from './types';

const API_URL = import.meta.env.VITE_API_URL ?? 'http://localhost:8000';

async function request<T>(path: string): Promise<T> {
  const response = await fetch(`${API_URL}${path}`);
  if (!response.ok) {
    const detail = await response.json().catch(() => ({}));
    throw new Error(detail.detail ?? `Request failed with status ${response.status}`);
  }
  return response.json() as Promise<T>;
}

export function getChampions() {
  return request<ChampionData>('/api/static/champions');
}

export function getItems() {
  return request<ItemData>('/api/static/items');
}

export function getBuilds(filters: BuildFilters) {
  const params = new URLSearchParams();
  if (filters.championId) params.set('championId', String(filters.championId));
  if (filters.role) params.set('role', filters.role);
  if (filters.opponentChampionId) params.set('opponentChampionId', String(filters.opponentChampionId));
  if (filters.patch) params.set('patch', filters.patch);
  if (filters.rankBucket) params.set('rankBucket', filters.rankBucket);
  params.set('minGames', String(filters.minGames));
  return request<AnalyticsBuildResponse>(`/api/analytics/builds?${params.toString()}`);
}

export function getLiveGame(gameName: string, tagLine: string, platform: string) {
  const params = new URLSearchParams({ gameName, tagLine, platform });
  return request<LiveGame>(`/api/live-game?${params.toString()}`);
}
