import { summonerPath } from './lookup';
import { winConditionFromSlug, winConditionPath, type WinConditionKey } from './winConditions';

export type AppRoute =
  | { kind: 'home' }
  | { kind: 'tier-list' }
  | { kind: 'win-conditions' }
  | { kind: 'win-condition-detail'; condition: WinConditionKey }
  | { kind: 'champion'; championSlug?: string }
  | { kind: 'summoner'; platform?: string; gameName?: string; tagLine?: string };

export function readRoute(): AppRoute {
  const parts = window.location.pathname.split('/').filter(Boolean).map((part) => decodeURIComponent(part));
  if (parts[0] === 'champions') {
    return { kind: 'champion', championSlug: parts[1] };
  }
  if (parts[0] === 'tier-list') {
    return { kind: 'tier-list' };
  }
  if (parts[0] === 'win-conditions' || parts[0] === 'winconditions') {
    return { kind: 'win-conditions' };
  }
  const winCondition = winConditionFromSlug(parts[0]);
  if (winCondition) {
    return { kind: 'win-condition-detail', condition: winCondition };
  }
  if (parts[0] === 'summoners') {
    return { kind: 'summoner', platform: parts[1], gameName: parts[2], tagLine: parts[3] };
  }
  return { kind: 'home' };
}

export function pathForRoute(route: AppRoute) {
  if (route.kind === 'champion') {
    return route.championSlug ? `/champions/${encodeURIComponent(route.championSlug)}` : '/champions';
  }
  if (route.kind === 'tier-list') {
    return '/tier-list';
  }
  if (route.kind === 'win-conditions') {
    return '/win-conditions';
  }
  if (route.kind === 'win-condition-detail') {
    return winConditionPath(route.condition);
  }
  if (route.kind === 'summoner') {
    if (!route.gameName) return '/summoners';
    return summonerPath(route.platform ?? 'NA1', route.gameName, route.tagLine);
  }
  return '/';
}

export function appShellClass(route: AppRoute) {
  const classes = ['app-shell'];
  if (route.kind === 'champion' || route.kind === 'tier-list' || route.kind === 'win-conditions' || route.kind === 'win-condition-detail') {
    classes.push('guide-mode');
  }
  if (route.kind === 'home') {
    classes.push('page-home', 'background-showcase');
  } else if (route.kind === 'summoner') {
    classes.push('page-summoner', 'background-dense');
  } else if (route.kind === 'tier-list') {
    classes.push('page-tier-list', 'background-data');
  } else if (route.kind === 'win-conditions' || route.kind === 'win-condition-detail') {
    classes.push('page-win-conditions', 'background-directory');
  } else if (route.championSlug) {
    classes.push('page-champion-guide', 'background-champion-scope');
  } else {
    classes.push('page-champion-index', 'background-directory');
  }
  return classes.join(' ');
}
