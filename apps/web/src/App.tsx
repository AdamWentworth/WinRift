import { useMutation, useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { BuildExplorer } from './components/BuildExplorer';
import { BuildResults } from './components/BuildResults';
import { LiveMatchPanel } from './components/LiveMatchPanel';
import { getBuilds, getChampions, getItems, getLiveGame } from './api/client';
import type { BuildFilters } from './api/types';

export function App() {
  const [filters, setFilters] = useState<BuildFilters>({ minGames: 5 });
  const [submittedFilters, setSubmittedFilters] = useState<BuildFilters>({ minGames: 5 });

  const champions = useQuery({ queryKey: ['champions'], queryFn: getChampions });
  const items = useQuery({ queryKey: ['items'], queryFn: getItems });
  const builds = useQuery({
    queryKey: ['builds', submittedFilters],
    queryFn: () => getBuilds(submittedFilters),
  });
  const liveGame = useMutation({
    mutationFn: ({ gameName, tagLine, platform }: { gameName: string; tagLine: string; platform: string }) => getLiveGame(gameName, tagLine, platform),
  });

  const useLiveContext = (nextFilters: BuildFilters) => {
    const merged = { ...filters, ...nextFilters };
    setFilters(merged);
    setSubmittedFilters(merged);
  };

  return (
    <main className="app-shell">
      <header className="topbar">
        <div>
          <h1>WinRift</h1>
          <p>Build matchup analytics for League of Legends ranked games.</p>
        </div>
        <div className="status-chip">ClickHouse MVP</div>
      </header>
      <div className="workspace">
        <div className="left-stack">
          <BuildExplorer
            champions={champions.data}
            filters={filters}
            onChange={setFilters}
            onSubmit={() => setSubmittedFilters(filters)}
          />
          <LiveMatchPanel
            champions={champions.data}
            liveGame={liveGame.data}
            loading={liveGame.isPending}
            error={liveGame.error}
            onSearch={(gameName, tagLine, platform) => liveGame.mutate({ gameName, tagLine, platform })}
            onUseContext={useLiveContext}
          />
        </div>
        <section className="panel results-panel">
          <div className="panel-heading">
            <h2>Contextual Patterns</h2>
            <span className="muted">{builds.data?.results.length ?? 0} rows</span>
          </div>
          <BuildResults builds={builds.data?.results ?? []} champions={champions.data} items={items.data} loading={builds.isLoading} />
        </section>
      </div>
    </main>
  );
}
