import type { Champion, ChampionData } from '../api/types';
import { championList } from './staticData';

export const platforms = [
  { value: 'NA1', label: 'NA', color: 'red' },
  { value: 'EUW1', label: 'EUW', color: 'blue' },
  { value: 'EUN1', label: 'EUNE', color: 'darkgreen' },
  { value: 'LA1', label: 'LAN', color: 'darkorange' },
  { value: 'LA2', label: 'LAS', color: 'firebrick' },
  { value: 'BR1', label: 'BR', color: 'forestgreen' },
  { value: 'TR1', label: 'TR', color: 'indigo' },
  { value: 'RU', label: 'RU', color: 'darkred' },
  { value: 'KR', label: 'KR', color: 'navy' },
  { value: 'JP1', label: 'JP', color: 'crimson' },
  { value: 'OC1', label: 'OCE', color: 'steelblue' },
] as const;

export function parseRiotId(value: string) {
  const trimmed = value.trim();
  const separator = trimmed.lastIndexOf('#');
  if (separator === -1) {
    return { gameName: trimmed, tagLine: '' };
  }
  return {
    gameName: trimmed.slice(0, separator).trim(),
    tagLine: trimmed.slice(separator + 1).trim(),
  };
}

export function platformLabel(value: string) {
  return platforms.find((candidate) => candidate.value === value)?.label ?? value;
}

export function normalizeLookup(value: string) {
  return value.toLowerCase().replace(/[^a-z0-9]/g, '');
}

export function championRouteSlug(champion: Champion) {
  return champion.id;
}

export function findChampionByLookup(champions: ChampionData | undefined, value: string) {
  const normalized = normalizeLookup(value);
  if (!normalized) return undefined;
  return championList(champions).find((champion) => (
    normalizeLookup(champion.name) === normalized ||
    normalizeLookup(champion.id) === normalized ||
    normalizeLookup(champion.key) === normalized
  ));
}

export function championIdFromRoute(champions: ChampionData | undefined, slug?: string) {
  const champion = findChampionByLookup(champions, slug ?? '');
  return champion ? Number(champion.key) : undefined;
}

export function summonerPath(platform: string, gameName: string, tagLine?: string) {
  const parts = ['summoners', platform, gameName];
  if (tagLine) {
    parts.push(tagLine);
  }
  return `/${parts.map((part) => encodeURIComponent(part)).join('/')}`;
}

export function championPath(champion: Champion) {
  return `/champions/${encodeURIComponent(championRouteSlug(champion))}`;
}
