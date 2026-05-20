import { useCallback, useMemo, useState, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { getChampionSplashes, getChampions, getItems, getRunes, getSummonerSpells } from './api/client';
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

type AppRoute =
  | { kind: 'home' }
  | { kind: 'tier-list' }
  | { kind: 'champion'; championSlug?: string }
  | { kind: 'summoner'; platform?: string; gameName?: string; tagLine?: string };

export function App() {
  const [route, setRoute] = useState<AppRoute>(() => readRoute());
  const champions = useQuery({ queryKey: ['champions'], queryFn: getChampions });
  const championSplashes = useQuery({ queryKey: ['champion-splashes'], queryFn: getChampionSplashes, staleTime: Infinity });
  const items = useQuery({ queryKey: ['items'], queryFn: getItems });
  const spells = useQuery({ queryKey: ['summoner-spells'], queryFn: getSummonerSpells });
  const runes = useQuery({ queryKey: ['runes'], queryFn: getRunes });

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
  const activeSection = route.kind === 'champion' ? 'champions' : route.kind === 'tier-list' ? 'tier-list' : route.kind === 'summoner' ? 'summoners' : 'home';
  const initialChampionId = useMemo(() => (
    route.kind === 'champion' ? championIdFromRoute(champions.data, route.championSlug) : undefined
  ), [champions.data, route]);

  return (
    <main className={route.kind === 'champion' || route.kind === 'tier-list' ? 'app-shell guide-mode' : 'app-shell'}>
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
          onSelectChampion={(champion) => navigate({ kind: 'champion', championSlug: championRouteSlug(champion) })}
        />
      ) : route.kind === 'champion' && !route.championSlug ? (
        <ChampionDirectoryPage
          champions={champions.data}
          onSelectChampion={(champion) => navigate({ kind: 'champion', championSlug: championRouteSlug(champion) })}
        />
      ) : route.kind === 'champion' ? (
        <BuildGuidePage
          champions={champions.data}
          items={items.data}
          spells={spells.data}
          runes={runes.data}
          initialChampionId={initialChampionId}
          onChampionChange={(champion) => navigate({ kind: 'champion', championSlug: championRouteSlug(champion) })}
        />
      ) : route.kind === 'summoner' ? (
        <SummonerProfilePage
          platform={route.platform}
          gameName={route.gameName}
          tagLine={route.tagLine}
          champions={champions.data}
          championSplashes={championSplashes.data}
          items={items.data}
          spells={spells.data}
          runes={runes.data}
          onUseAlias={(alias) => navigate({ kind: 'summoner', platform: alias.platform, gameName: alias.gameName, tagLine: alias.tagLine })}
          onResolvedAlias={(alias) => navigate({ kind: 'summoner', platform: alias.platform, gameName: alias.gameName, tagLine: alias.tagLine }, { replace: true })}
        />
      ) : (
        <LiveMatchPanel
          champions={champions.data}
          championSplashes={championSplashes.data}
          items={items.data}
          spells={spells.data}
          runes={runes.data}
          loading={false}
          onSearch={(gameName, tagLine, platform) => navigate({ kind: 'summoner', platform, gameName, tagLine })}
          onChampionSearch={(champion) => navigate({ kind: 'champion', championSlug: championRouteSlug(champion) })}
        />
      )}
    </main>
  );
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
