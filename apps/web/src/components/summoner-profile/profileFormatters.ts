import type { ChampionData, SummonerProfile } from '../../api/types';
import { championByKey } from '../../lib/staticData';

export type ProfileFreshness = {
  tone: 'fresh' | 'recent' | 'stale' | 'empty';
  label: string;
  body: string;
  snapshotDetail: string;
};

export function formatNumber(value: number) {
  return new Intl.NumberFormat('en-US').format(value);
}

export function championName(champions: ChampionData | undefined, championId: number) {
  return championByKey(champions, championId)?.name ?? `Champion ${championId}`;
}

export function formatProfileDate(value?: string) {
  if (!value) return '--';
  const date = new Date(value);
  if (Number.isNaN(date.getTime()) || date.getFullYear() <= 1970) return '--';
  return new Intl.DateTimeFormat('en-US', { month: 'short', day: 'numeric' }).format(date);
}

export function profileFreshness(summary: SummonerProfile['summary']): ProfileFreshness {
  const days = daysSinceDate(summary.lastSeen);
  const firstSeen = formatProfileDate(summary.firstSeen);
  const lastSeen = formatProfileDate(summary.lastSeen);
  if (days === undefined) {
    return {
      tone: 'empty',
      label: 'No stored window yet',
      body: 'The collector has not attached retained ranked games to this profile yet. Live lookup can still work if the player is in game.',
      snapshotDetail: 'No retained games yet',
    };
  }
  const relativeLastSeen = relativeDayLabel(days);
  const sampleText = `${formatNumber(summary.games)} stored ${summary.games === 1 ? 'game' : 'games'}`;
  const firstSeenDetail = firstSeen !== '--' ? ` · first ${firstSeen}` : '';
  if (days <= 2) {
    return {
      tone: 'fresh',
      label: 'Fresh stored sample',
      body: `Last stored game was ${relativeLastSeen}. This profile is using ${sampleText}${firstSeen !== '--' ? ` since ${firstSeen}` : ''}.`,
      snapshotDetail: `${relativeLastSeen}${firstSeenDetail}`,
    };
  }
  if (days <= 14) {
    return {
      tone: 'recent',
      label: 'Recent stored sample',
      body: `Last stored game was ${relativeLastSeen}. Treat form and champion comfort as recent stored history, not live-season truth.`,
      snapshotDetail: `${relativeLastSeen}${firstSeenDetail}`,
    };
  }
  return {
    tone: 'stale',
    label: 'Aging stored sample',
    body: `Last stored game was ${relativeLastSeen} on ${lastSeen}. This profile may lag behind the player's current form until the collector sees newer games.`,
    snapshotDetail: `${relativeLastSeen}${firstSeenDetail}`,
  };
}

export function formatGameDate(timestamp: number) {
  if (!timestamp) return 'unknown date';
  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) return 'unknown date';
  return new Intl.DateTimeFormat('en-US', { month: 'short', day: 'numeric' }).format(date);
}

export function formatDuration(seconds: number) {
  if (!seconds) return '--';
  const minutes = Math.floor(seconds / 60);
  const remaining = seconds % 60;
  return `${minutes}:${String(remaining).padStart(2, '0')}`;
}

function daysSinceDate(value?: string) {
  if (!value) return undefined;
  const date = new Date(value);
  if (Number.isNaN(date.getTime()) || date.getFullYear() <= 1970) {
    return undefined;
  }
  const now = Date.now();
  const diff = now - date.getTime();
  if (diff < 0) {
    return 0;
  }
  return Math.floor(diff / 86_400_000);
}

function relativeDayLabel(days: number) {
  if (days <= 0) return 'today';
  if (days === 1) return 'yesterday';
  return `${formatNumber(days)} days ago`;
}
