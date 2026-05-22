import { useCallback, useMemo, useState, useEffect } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getBuildAdvice, getChampionGuide, getChampionGuideIndex, getChampionRoleRates, getChampionSplashes, getChampions, getItems, getRunes, getSummonerSpells } from './api/client';
import type { Champion, ChampionRoleRate } from './api/types';
import { GlobalBackgroundStage } from './components/GlobalBackgroundStage';
import { BuildGuidePage } from './components/BuildGuidePage';
import { ChampionDirectoryPage } from './components/ChampionDirectoryPage';
import { LiveMatchPanel } from './components/LiveMatchPanel';
import { SummonerProfilePage } from './components/SummonerProfilePage';
import { TierListPage } from './components/TierListPage';
import {
  championIdFromRoute,
  championRouteSlug,
  summonerPath,
} from './lib/lookup';
import { normalizeRole } from './lib/roles';
import { championImageUrl, championSplashUrl } from './lib/staticData';

const DEFAULT_QUEUE_ID = 420;
const STATIC_STALE_TIME = Infinity;
const GUIDE_STALE_TIME = 10 * 60 * 1000;
const ROLE_STALE_TIME = 30 * 60 * 1000;

type AppRoute =
  | { kind: 'home' }
  | { kind: 'tier-list' }
  | { kind: 'champion'; championSlug?: string }
  | { kind: 'summoner'; platform?: string; gameName?: string; tagLine?: string };

export function App() {
  const queryClient = useQueryClient();
  const [route, setRoute] = useState<AppRoute>(() => readRoute());
  const [summonerBackgroundChampionIds, setSummonerBackgroundChampionIds] = useState<number[]>([]);
  const champions = useQuery({ queryKey: ['champions'], queryFn: getChampions, staleTime: STATIC_STALE_TIME });
  const championSplashes = useQuery({ queryKey: ['champion-splashes'], queryFn: getChampionSplashes, staleTime: STATIC_STALE_TIME });
  const items = useQuery({ queryKey: ['items'], queryFn: getItems, staleTime: STATIC_STALE_TIME });
  const spells = useQuery({ queryKey: ['summoner-spells'], queryFn: getSummonerSpells, staleTime: STATIC_STALE_TIME });
  const runes = useQuery({ queryKey: ['runes'], queryFn: getRunes, staleTime: STATIC_STALE_TIME });
  const championIds = useMemo(() => {
    if (!champions.data) return [];
    return Object.values(champions.data.data.data)
      .map((champion) => Number(champion.key))
      .filter((championId) => Number.isFinite(championId) && championId > 0);
  }, [champions.data]);
  const allChampionRoleRates = useQuery({
    queryKey: ['champion-main-roles', DEFAULT_QUEUE_ID, championIds.join(',')],
    queryFn: () => getChampionRoleRates(championIds, DEFAULT_QUEUE_ID),
    enabled: championIds.length > 0,
    staleTime: ROLE_STALE_TIME,
  });
  const allChampionRoleRateRows = useMemo(() => allChampionRoleRates.data?.results ?? [], [allChampionRoleRates.data?.results]);

  useEffect(() => {
    const onPopState = () => setRoute(readRoute());
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  }, []);

  const navigate = useCallback((nextRoute: AppRoute, options?: { replace?: boolean }) => {
    const path = pathForRoute(nextRoute);
    if (window.location.pathname !== path) {
      if (options?.replace) {
        window.history.replaceState({}, '', path);
      } else {
        window.history.pushState({}, '', path);
      }
    }
    setRoute(nextRoute);
  }, []);

  const goHome = useCallback(() => navigate({ kind: 'home' }), [navigate]);
  const prefetchChampionGuide = useCallback((champion: Champion, preferredRole?: string) => {
    const championId = Number(champion.key);
    const patch = patchBucketFromVersion(champions.data?.version);
    if (!championId || !patch) return;

    warmImage(championImageUrl(champions.data, championId));
    warmImage(championSplashUrl(champions.data, championId));

    const prefetchForRole = (roleValue?: string) => {
      const role = normalizeRole(roleValue ?? '') || 'MIDDLE';
      const itemContext = itemContextForRole(role);
      void queryClient.prefetchQuery({
        queryKey: ['champion-guide', championId, role, patch, ''],
        queryFn: () => getChampionGuide({ championId, role, patch, rankBucket: '', minGames: 5, limit: 12 }),
        staleTime: GUIDE_STALE_TIME,
      });
      void queryClient.prefetchQuery({
        queryKey: ['guide-build-advice', championId, role, patch, '', 0],
        queryFn: () => getBuildAdvice({
          championId,
          role,
          itemContext,
          patch,
          rankBucket: '',
          minGames: 5,
          championMinGames: 10,
          limit: 4,
        }),
        staleTime: GUIDE_STALE_TIME,
      });
      void queryClient.prefetchQuery({
        queryKey: ['champion-guide-index', role, patch, ''],
        queryFn: () => getChampionGuideIndex({ role, patch, rankBucket: '', minGames: 1, limit: 250 }),
        staleTime: GUIDE_STALE_TIME,
      });
    };

    const normalizedPreferredRole = normalizeRole(preferredRole ?? '');
    const cachedDetectedRole = mainChampionRole(allChampionRoleRateRows, championId);
    if (normalizedPreferredRole) {
      prefetchForRole(normalizedPreferredRole);
    } else if (cachedDetectedRole) {
      prefetchForRole(cachedDetectedRole);
    }

    if (cachedDetectedRole) return;

    void queryClient.ensureQueryData({
      queryKey: ['champion-main-role', championId],
      queryFn: () => getChampionRoleRates([championId], DEFAULT_QUEUE_ID),
      staleTime: ROLE_STALE_TIME,
    }).then((data) => {
      const detectedRole = mainChampionRole(data.results ?? [], championId);
      if (!normalizedPreferredRole || detectedRole !== normalizedPreferredRole) {
        prefetchForRole(detectedRole);
      }
    }).catch(() => {
      if (!normalizedPreferredRole) {
        prefetchForRole('MIDDLE');
      }
    });
  }, [allChampionRoleRateRows, champions.data, queryClient]);
  const openChampionGuide = useCallback((champion: Champion, preferredRole?: string) => {
    prefetchChampionGuide(champion, preferredRole);
    navigate({ kind: 'champion', championSlug: championRouteSlug(champion) });
  }, [navigate, prefetchChampionGuide]);
  const activeSection = route.kind === 'champion' ? 'champions' : route.kind === 'tier-list' ? 'tier-list' : route.kind === 'summoner' ? 'summoners' : 'home';
  const initialChampionId = useMemo(() => (
    route.kind === 'champion' ? championIdFromRoute(champions.data, route.championSlug) : undefined
  ), [champions.data, route]);
  const backgroundChampionScopeId = route.kind === 'champion' && route.championSlug ? initialChampionId : undefined;
  const backgroundChampionScopeIds = route.kind === 'summoner' ? summonerBackgroundChampionIds : undefined;
  const appShellClassName = appShellClass(route);

  useEffect(() => {
    if (route.kind !== 'summoner') {
      setSummonerBackgroundChampionIds([]);
    }
  }, [route.kind]);

  return (
    <main className={appShellClassName}>
      <GlobalBackgroundStage
        champions={champions.data}
        championSplashes={championSplashes.data}
        championScopeId={backgroundChampionScopeId}
        championScopeIds={backgroundChampionScopeIds}
      />
      <header className="topbar">
        <div className="topbar-brand">
          <h1>
            <button className="topbar-logo" onClick={goHome} type="button" aria-label="WinRift home">
              WinRift
            </button>
          </h1>
        </div>
        <nav className="topbar-nav" aria-label="Primary">
          <button className={activeSection === 'home' ? 'selected' : ''} onClick={goHome} type="button">Home</button>
          <button
            className={activeSection === 'champions' ? 'selected' : ''}
            onClick={() => navigate({ kind: 'champion' })}
            type="button"
          >
            Champions
          </button>
          <button
            className={activeSection === 'tier-list' ? 'selected' : ''}
            onClick={() => navigate({ kind: 'tier-list' })}
            type="button"
          >
            Tier List
          </button>
          <button
            className={activeSection === 'summoners' ? 'selected' : ''}
            onClick={() => navigate({ kind: 'summoner' })}
            type="button"
          >
            Summoners
          </button>
        </nav>
      </header>

      {route.kind === 'tier-list' ? (
        <TierListPage
          champions={champions.data}
          onChampionIntent={prefetchChampionGuide}
          onSelectChampion={openChampionGuide}
        />
      ) : route.kind === 'champion' && !route.championSlug ? (
        <ChampionDirectoryPage
          champions={champions.data}
          onChampionIntent={prefetchChampionGuide}
          onSelectChampion={openChampionGuide}
        />
      ) : route.kind === 'champion' ? (
        <BuildGuidePage
          champions={champions.data}
          items={items.data}
          spells={spells.data}
          runes={runes.data}
          initialChampionId={initialChampionId}
          roleRates={allChampionRoleRateRows}
          onChampionChange={openChampionGuide}
        />
      ) : route.kind === 'summoner' ? (
        <SummonerProfilePage
          platform={route.platform}
          gameName={route.gameName}
          tagLine={route.tagLine}
          champions={champions.data}
          items={items.data}
          spells={spells.data}
          runes={runes.data}
          onUseAlias={(alias) => navigate({ kind: 'summoner', platform: alias.platform, gameName: alias.gameName, tagLine: alias.tagLine })}
          onResolvedAlias={(alias) => navigate({ kind: 'summoner', platform: alias.platform, gameName: alias.gameName, tagLine: alias.tagLine }, { replace: true })}
          onBackgroundChampionScopeChange={setSummonerBackgroundChampionIds}
        />
      ) : (
        <LiveMatchPanel
          champions={champions.data}
          items={items.data}
          spells={spells.data}
          runes={runes.data}
          loading={false}
          onSearch={(gameName, tagLine, platform) => navigate({ kind: 'summoner', platform, gameName, tagLine })}
          onChampionSearch={openChampionGuide}
        />
      )}
    </main>
  );
}

function patchBucketFromVersion(version?: string) {
  const parts = (version ?? '').split('.');
  if (parts.length >= 2) {
    return `${parts[0]}.${parts[1]}`;
  }
  return '';
}

function itemContextForRole(role: string): 'JUNGLE' | 'SUPPORT' | undefined {
  if (role === 'JUNGLE') return 'JUNGLE';
  if (role === 'UTILITY') return 'SUPPORT';
  return undefined;
}

function mainChampionRole(rows: ChampionRoleRate[], championId: number) {
  const ranked = rows
    .filter((row) => row.championId === championId && normalizeRole(row.role))
    .sort((a, b) => {
      if (a.games !== b.games) return b.games - a.games;
      return b.pickRate - a.pickRate;
    });
  return normalizeRole(ranked[0]?.role);
}

function warmImage(src: string) {
  if (!src) return;
  const image = new Image();
  image.decoding = 'async';
  image.src = src;
}

function appShellClass(route: AppRoute) {
  const classes = ['app-shell'];
  if (route.kind === 'champion' || route.kind === 'tier-list') {
    classes.push('guide-mode');
  }
  if (route.kind === 'home') {
    classes.push('page-home', 'background-showcase');
  } else if (route.kind === 'summoner') {
    classes.push('page-summoner', 'background-dense');
  } else if (route.kind === 'tier-list') {
    classes.push('page-tier-list', 'background-data');
  } else if (route.championSlug) {
    classes.push('page-champion-guide', 'background-champion-scope');
  } else {
    classes.push('page-champion-index', 'background-directory');
  }
  return classes.join(' ');
}

function readRoute(): AppRoute {
  const parts = window.location.pathname.split('/').filter(Boolean).map((part) => decodeURIComponent(part));
  if (parts[0] === 'champions') {
    return { kind: 'champion', championSlug: parts[1] };
  }
  if (parts[0] === 'tier-list') {
    return { kind: 'tier-list' };
  }
  if (parts[0] === 'summoners') {
    return { kind: 'summoner', platform: parts[1], gameName: parts[2], tagLine: parts[3] };
  }
  return { kind: 'home' };
}

function pathForRoute(route: AppRoute) {
  if (route.kind === 'champion') {
    return route.championSlug ? `/champions/${encodeURIComponent(route.championSlug)}` : '/champions';
  }
  if (route.kind === 'tier-list') {
    return '/tier-list';
  }
  if (route.kind === 'summoner') {
    if (!route.gameName) return '/summoners';
    return summonerPath(route.platform ?? 'NA1', route.gameName, route.tagLine);
  }
  return '/';
}
