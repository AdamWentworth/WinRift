import { useMutation, useQuery } from '@tanstack/react-query';
import { LiveMatchPanel } from './components/LiveMatchPanel';
import { getChampions, getItems, getLiveGame, getRunes, getSummonerSpells } from './api/client';

export function App() {
  const champions = useQuery({ queryKey: ['champions'], queryFn: getChampions });
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

  const hasLiveGame = Boolean(!liveGame.isError && liveGame.data);

  return (
    <main className={hasLiveGame ? 'app-shell live-mode' : 'app-shell'}>
      <header className={hasLiveGame ? 'topbar live-topbar' : 'topbar'}>
        <div>
          <h1>WinRift</h1>
        </div>
      </header>
      <LiveMatchPanel
        champions={champions.data}
        items={items.data}
        spells={spells.data}
        runes={runes.data}
        liveGame={liveGame.isError ? undefined : liveGame.data}
        loading={liveGame.isPending}
        error={liveGame.error}
        onSearch={searchLiveGame}
      />
    </main>
  );
}
