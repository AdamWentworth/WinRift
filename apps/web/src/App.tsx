import { lazy, Suspense, useCallback, useEffect, useMemo, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { getAnalyticsPatches, getChampionPageBundle, getChampionSplashes, getChampions, getItems, getRunes, getSummonerSpells } from './api/client';
import type { Champion } from './api/types';
import { GlobalBackgroundStage } from './components/GlobalBackgroundStage';
import { HeaderSearch } from './components/HeaderSearch';
import {
  championIdFromRoute,
  championRouteSlug,
} from './lib/lookup';
import { recommendedAnalyticsPatch, storedAnalyticsPatch, storeAnalyticsPatch } from './lib/analyticsPatch';
import { appShellClass, pathForRoute, readRoute, type AppRoute } from './lib/appRouting';
import { itemContextForRole } from './lib/championRoles';
import { patchBucketFromVersion } from './lib/patches';
import { queryGcTime, queryStaleTime } from './lib/queryPolicies';
import { CHAMPION_PAGE_QUERY_VERSION } from './lib/queryVersions';
import { normalizeRole } from './lib/roles';
import { championImageUrl, championList, championSplashUrl } from './lib/staticData';
import { WIN_CONDITION_DEFINITIONS, type WinConditionKey } from './lib/winConditions';

const DEFAULT_QUEUE_ID = 420;
const BACKGROUND_SPLASH_CATALOG_DELAY_MS = 10_000;

const BuildGuidePage = lazy(() => import('./components/BuildGuidePage').then((module) => ({ default: module.BuildGuidePage })));
const ChampionDirectoryPage = lazy(() => import('./components/ChampionDirectoryPage').then((module) => ({ default: module.ChampionDirectoryPage })));
const LiveMatchPanel = lazy(() => import('./components/LiveMatchPanel').then((module) => ({ default: module.LiveMatchPanel })));
const SummonerProfilePage = lazy(() => import('./components/SummonerProfilePage').then((module) => ({ default: module.SummonerProfilePage })));
const TierListPage = lazy(() => import('./components/TierListPage').then((module) => ({ default: module.TierListPage })));
const WinConditionsPage = lazy(() => import('./components/win-conditions/WinConditionsPage').then((module) => ({ default: module.WinConditionsPage })));
const WinConditionDetailPage = lazy(() => import('./components/win-conditions/WinConditionDetailPage').then((module) => ({ default: module.WinConditionDetailPage })));

export function App() {
  const queryClient = useQueryClient();
  const [route, setRoute] = useState<AppRoute>(() => readRoute());
  const [selectedAnalyticsPatch, setSelectedAnalyticsPatch] = useState(() => storedAnalyticsPatch());
  const [summonerBackgroundChampionIds, setSummonerBackgroundChampionIds] = useState<number[]>([]);
  const [backgroundSplashCatalogEnabled, setBackgroundSplashCatalogEnabled] = useState(false);
  const needsGameMetadata = (route.kind === 'summoner' && Boolean(route.gameName)) || (route.kind === 'champion' && Boolean(route.championSlug));
  const needsWinConditionDetailBackground = route.kind === 'win-condition-detail';
  const champions = useQuery({ queryKey: ['champions'], queryFn: ({ signal }) => getChampions({ signal }), staleTime: queryStaleTime.static, gcTime: queryGcTime.static });
  const analyticsPatches = useQuery({
    queryKey: ['analytics-patches', DEFAULT_QUEUE_ID],
    queryFn: ({ signal }) => getAnalyticsPatches(DEFAULT_QUEUE_ID, { signal }),
    staleTime: queryStaleTime.patchList,
  });
  const championSplashes = useQuery({
    queryKey: ['champion-splashes'],
    queryFn: ({ signal }) => getChampionSplashes({ signal }),
    enabled: backgroundSplashCatalogEnabled || needsWinConditionDetailBackground,
    staleTime: queryStaleTime.static,
    gcTime: queryGcTime.static,
  });
  const items = useQuery({ queryKey: ['items'], queryFn: ({ signal }) => getItems({ signal }), enabled: needsGameMetadata, staleTime: queryStaleTime.static, gcTime: queryGcTime.static });
  const spells = useQuery({ queryKey: ['summoner-spells'], queryFn: ({ signal }) => getSummonerSpells({ signal }), enabled: needsGameMetadata, staleTime: queryStaleTime.static, gcTime: queryGcTime.static });
  const runes = useQuery({ queryKey: ['runes'], queryFn: ({ signal }) => getRunes({ signal }), enabled: needsGameMetadata, staleTime: queryStaleTime.static, gcTime: queryGcTime.static });
  const staticPatch = useMemo(() => patchBucketFromVersion(champions.data?.version), [champions.data?.version]);
  const patchOptions = analyticsPatches.data?.results ?? [];
  const activeAnalyticsPatch = useMemo(() => {
    const selected = patchOptions.find((patch) => patch.patch === selectedAnalyticsPatch);
    if (selected) return selected.patch;
    if (!patchOptions.length && !analyticsPatches.isError) {
      return selectedAnalyticsPatch || '';
    }
    return recommendedAnalyticsPatch(patchOptions, staticPatch) || staticPatch;
  }, [analyticsPatches.isError, patchOptions, selectedAnalyticsPatch, staticPatch]);

  useEffect(() => {
    const onPopState = () => setRoute(readRoute());
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => setBackgroundSplashCatalogEnabled(true), BACKGROUND_SPLASH_CATALOG_DELAY_MS);
    return () => window.clearTimeout(timer);
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
  const updateAnalyticsPatch = useCallback((patch: string) => {
    setSelectedAnalyticsPatch(patch);
    storeAnalyticsPatch(patch);
  }, []);
  const prefetchChampionGuide = useCallback((champion: Champion, preferredRole?: string) => {
    const championId = Number(champion.key);
    const patch = activeAnalyticsPatch;
    if (!championId || !patch) return;

    warmImage(championImageUrl(champions.data, championId));
    warmImage(championSplashUrl(champions.data, championId));

    const prefetchForRole = (roleValue?: string) => {
      const role = normalizeRole(roleValue ?? '');
      const itemContext = itemContextForRole(role);
      void queryClient.prefetchQuery({
        queryKey: ['champion-page', CHAMPION_PAGE_QUERY_VERSION, championId, role || 'AUTO', patch, '', 0],
        queryFn: ({ signal }) => getChampionPageBundle({
          championId,
          role: role || undefined,
          itemContext,
          patch,
          rankBucket: '',
          minGames: 5,
          championMinGames: 10,
          guideMinGames: 5,
          guideLimit: 12,
          indexMinGames: 1,
          indexLimit: 250,
          queueId: DEFAULT_QUEUE_ID,
          limit: 4,
        }, { signal }),
        staleTime: queryStaleTime.championGuide,
      });
    };

    const normalizedPreferredRole = normalizeRole(preferredRole ?? '');
    if (normalizedPreferredRole) {
      prefetchForRole(normalizedPreferredRole);
      return;
    }
    prefetchForRole();
  }, [activeAnalyticsPatch, champions.data, queryClient]);
  const openChampionGuide = useCallback((champion: Champion, preferredRole?: string) => {
    prefetchChampionGuide(champion, preferredRole);
    navigate({ kind: 'champion', championSlug: championRouteSlug(champion) });
  }, [navigate, prefetchChampionGuide]);
  const activeSection = route.kind === 'champion'
    ? 'champions'
    : route.kind === 'tier-list'
      ? 'tier-list'
      : route.kind === 'win-conditions' || route.kind === 'win-condition-detail'
        ? 'win-conditions'
        : route.kind === 'summoner'
          ? 'summoners'
          : 'home';
  const initialChampionId = useMemo(() => (
    route.kind === 'champion' ? championIdFromRoute(champions.data, route.championSlug) : undefined
  ), [champions.data, route]);
  const winConditionBackgroundChampionIds = useMemo(() => (
    route.kind === 'win-condition-detail'
      ? championIdsForWinConditionBackground(champions.data, route.condition)
      : undefined
  ), [champions.data, route]);
  const backgroundChampionScopeId = route.kind === 'champion' && route.championSlug ? initialChampionId : undefined;
  const backgroundChampionScopeIds = route.kind === 'summoner' ? summonerBackgroundChampionIds : winConditionBackgroundChampionIds;
  const appShellClassName = appShellClass(route);
  const showHeaderSearch = route.kind === 'champion'
    || route.kind === 'tier-list'
    || route.kind === 'win-conditions'
    || route.kind === 'win-condition-detail'
    || (route.kind === 'summoner' && Boolean(route.gameName));

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
        strictChampionScope={route.kind === 'win-condition-detail'}
      />
      <header className="topbar">
        <div className="topbar-brand">
          <h1>
            <button className="topbar-logo" onClick={goHome} type="button" aria-label="WinRift home">
              <img className="topbar-logo-icon" src="/images/brand/winrift-icon.png" alt="" aria-hidden="true" />
              <img className="topbar-logo-wordmark" src="/images/brand/winrift-wordmark.png" alt="" aria-hidden="true" />
              <span className="sr-only">WinRift</span>
            </button>
          </h1>
        </div>
        {showHeaderSearch ? (
          <HeaderSearch
            champions={champions.data}
            onSearch={(gameName, tagLine, platform) => navigate({ kind: 'summoner', platform, gameName, tagLine })}
            onChampionSearch={openChampionGuide}
          />
        ) : null}
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
            className={activeSection === 'win-conditions' ? 'selected' : ''}
            onClick={() => navigate({ kind: 'win-conditions' })}
            type="button"
          >
            Win Conditions
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

      <Suspense fallback={<RouteFallback />}>
        {route.kind === 'win-conditions' ? (
          <WinConditionsPage
            champions={champions.data}
            onSelectCondition={(condition) => navigate({ kind: 'win-condition-detail', condition })}
            onSelectChampion={openChampionGuide}
          />
        ) : route.kind === 'win-condition-detail' ? (
          <WinConditionDetailPage
            champions={champions.data}
            condition={route.condition}
            onBack={() => navigate({ kind: 'win-conditions' })}
            onSelectChampion={openChampionGuide}
            onSelectCondition={(condition) => navigate({ kind: 'win-condition-detail', condition })}
          />
        ) : route.kind === 'tier-list' ? (
          <TierListPage
            champions={champions.data}
            analyticsPatch={activeAnalyticsPatch}
            analyticsPatchLoading={analyticsPatches.isLoading}
            analyticsPatchOptions={patchOptions}
            currentAnalyticsPatch={analyticsPatches.data?.currentPatch || staticPatch}
            onAnalyticsPatchChange={updateAnalyticsPatch}
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
            analyticsPatch={activeAnalyticsPatch}
            analyticsPatchLoading={analyticsPatches.isLoading}
            analyticsPatchOptions={patchOptions}
            currentAnalyticsPatch={analyticsPatches.data?.currentPatch || staticPatch}
            onAnalyticsPatchChange={updateAnalyticsPatch}
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
            analyticsPatch={activeAnalyticsPatch}
            onUseAlias={(alias) => navigate({ kind: 'summoner', platform: alias.platform, gameName: alias.gameName, tagLine: alias.tagLine })}
            onSearch={(gameName, tagLine, platform) => navigate({ kind: 'summoner', platform, gameName, tagLine })}
            onResolvedAlias={(alias) => navigate({ kind: 'summoner', platform: alias.platform, gameName: alias.gameName, tagLine: alias.tagLine }, { replace: true })}
            onBackgroundChampionScopeChange={setSummonerBackgroundChampionIds}
          />
        ) : (
          <LiveMatchPanel
            champions={champions.data}
            items={items.data}
            spells={spells.data}
            runes={runes.data}
            analyticsPatch={activeAnalyticsPatch}
            loading={false}
            onSearch={(gameName, tagLine, platform) => navigate({ kind: 'summoner', platform, gameName, tagLine })}
            onChampionSearch={openChampionGuide}
          />
        )}
      </Suspense>
    </main>
  );
}

function warmImage(src: string) {
  if (!src) return;
  const image = new Image();
  image.decoding = 'async';
  image.src = src;
}

function championIdsForWinConditionBackground(champions: Parameters<typeof championList>[0], condition: WinConditionKey) {
  const definition = WIN_CONDITION_DEFINITIONS.find((candidate) => candidate.key === condition);
  if (!definition) return [];

  const conditionChampionNames = [
    ...definition.examples,
    ...(definition.carryExamples ?? []),
    ...(definition.protectorExamples ?? []),
  ];
  const normalizedNames = new Set(conditionChampionNames.map((name) => name.toLowerCase()));
  return championList(champions)
    .filter((champion) => normalizedNames.has(champion.name.toLowerCase()))
    .map((champion) => Number(champion.key))
    .filter((championId) => Number.isFinite(championId));
}

function RouteFallback() {
  return (
    <section className="route-loading-panel" aria-live="polite">
      <img src="/images/brand/winrift-icon.png" alt="" aria-hidden="true" />
      <span>Loading WinRift</span>
    </section>
  );
}
