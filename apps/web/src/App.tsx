import { useMutation, useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { LiveMatchPanel } from './components/LiveMatchPanel';
import { getChampions, getChampionSplashes, getItems, getLiveGame, getRunes, getSummonerSpells } from './api/client';
import { BuildGuidePage } from './components/BuildGuidePage';

type AppView = 'live' | 'guides';

export function App() {
  const [view, setView] = useState<AppView>('live');
  const champions = useQuery({ queryKey: ['champions'], queryFn: getChampions });
  const championSplashes = useQuery({ queryKey: ['champion-splashes'], queryFn: getChampionSplashes, staleTime: Infinity });
  const items = useQuery({ queryKey: ['items'], queryFn: getItems });
  const spells = useQuery({ queryKey: ['summoner-spells'], queryFn: getSummonerSpells });
  const runes = useQuery({ queryKey: ['runes'], queryFn: getRunes });
  const liveGame = useMutation({
    mutationFn: ({ gameName, tagLine, platform }: { gameName: string; tagLine: string; platform: string }) => getLiveGame(gameName, tagLine, platform),
  });

  const searchLiveGame = (gameName: string, tagLine: string, platform: string) => {
    liveGame.reset();
    liveGame.mutate({ gameName, tagLine, platform });
  };

  const goHome = () => {
    setView('live');
    liveGame.reset();
  };

  const hasLiveGame = Boolean(view === 'live' && !liveGame.isError && liveGame.data);
  const showGuides = view === 'guides';

  return (
    <main className={showGuides ? 'app-shell guide-mode' : hasLiveGame ? 'app-shell live-mode' : 'app-shell'}>
      <header className={hasLiveGame ? 'topbar live-topbar' : 'topbar'}>
        <div>
          <h1>
            <button className="topbar-logo" onClick={goHome} type="button" aria-label="WinRift home">
              WinRift
            </button>
          </h1>
        </div>
        <nav className="topbar-nav" aria-label="Primary">
          <button className={view === 'live' ? 'selected' : ''} onClick={goHome} type="button">Live Lookup</button>
          <button
            className={showGuides ? 'selected' : ''}
            onClick={() => {
              liveGame.reset();
              setView('guides');
            }}
            type="button"
          >
            Build Guides
          </button>
        </nav>
      </header>
      {showGuides ? (
        <BuildGuidePage
          champions={champions.data}
          items={items.data}
          spells={spells.data}
          runes={runes.data}
        />
      ) : (
        <LiveMatchPanel
          champions={champions.data}
          championSplashes={championSplashes.data}
          items={items.data}
          spells={spells.data}
          runes={runes.data}
          liveGame={liveGame.isError ? undefined : liveGame.data}
          loading={liveGame.isPending}
          error={liveGame.error}
          onSearch={searchLiveGame}
        />
      )}
    </main>
  );
}
