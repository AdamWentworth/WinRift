import { Search } from 'lucide-react';
import { useMemo, useState } from 'react';
import type { CSSProperties } from 'react';
import type { Champion, ChampionData } from '../api/types';
import { normalizeLookup } from '../lib/lookup';
import { championImageUrl, championList, championSplashUrl } from '../lib/staticData';
import { MetricTile } from './ui/MetricTile';
import { SelectControl } from './ui/SelectControl';

const sortModes = [
  { value: 'name-asc', label: 'Name A-Z' },
  { value: 'name-desc', label: 'Name Z-A' },
  { value: 'title-asc', label: 'Title A-Z' },
];

type SortMode = typeof sortModes[number]['value'];

type Props = {
  champions?: ChampionData;
  onSelectChampion: (champion: Champion) => void;
};

export function ChampionDirectoryPage({ champions, onSelectChampion }: Props) {
  const [searchText, setSearchText] = useState('');
  const [sortMode, setSortMode] = useState<SortMode>('name-asc');
  const championsByName = useMemo(() => championList(champions), [champions]);
  const normalizedSearch = normalizeLookup(searchText);
  const filteredChampions = useMemo(() => {
    return championsByName
      .filter((champion) => {
        if (!normalizedSearch) return true;
        return normalizeLookup(champion.name).includes(normalizedSearch) || normalizeLookup(champion.id).includes(normalizedSearch);
      })
      .sort((a, b) => sortChampions(a, b, sortMode));
  }, [championsByName, normalizedSearch, sortMode]);

  return (
    <section className="champion-directory-page">
      <div className="champion-directory-hero">
        <div>
          <span>Champion Index</span>
          <h2>All Champions</h2>
          <p>Open a champion guide for collected builds, runes, skill paths, and matchup context. Role-based rankings belong in the tier-list view we can add separately.</p>
        </div>
        <div className="champion-directory-summary">
          <DirectorySummaryStat label="Champions" value={formatNumber(championsByName.length)} />
          <DirectorySummaryStat label="Patch" value={champions?.version ?? 'Current'} />
          <DirectorySummaryStat label="Sort" value={sortModes.find((mode) => mode.value === sortMode)?.label ?? 'Name A-Z'} />
        </div>
      </div>

      <div className="champion-directory-toolbar neutral">
        <label className="champion-directory-search">
          <Search size={16} />
          <input
            value={searchText}
            onChange={(event) => setSearchText(event.target.value)}
            placeholder="Search champions"
          />
        </label>
        <SelectControl className="guide-select-control compact" label="Sort" value={sortMode} onChange={(value) => setSortMode(value as SortMode)}>
          {sortModes.map((candidate) => (
            <option key={candidate.value} value={candidate.value}>{candidate.label}</option>
          ))}
        </SelectControl>
      </div>

      <div className="champion-directory-meta">
        <span>{formatNumber(filteredChampions.length)} champions shown</span>
        <b>{sortModes.find((mode) => mode.value === sortMode)?.label ?? 'Name A-Z'}</b>
      </div>

      <div className="champion-directory-grid neutral">
        {filteredChampions.map((champion) => (
          <ChampionDirectoryCard
            key={champion.key}
            champion={champion}
            champions={champions}
            onSelect={() => onSelectChampion(champion)}
          />
        ))}
      </div>
    </section>
  );
}

function ChampionDirectoryCard({ champion, champions, onSelect }: { champion: Champion; champions?: ChampionData; onSelect: () => void }) {
  return (
    <button
      className="champion-directory-card neutral"
      onClick={onSelect}
      style={{ '--champion-splash': `url(${championSplashUrl(champions, Number(champion.key))})` } as CSSProperties}
      type="button"
    >
      <img src={championImageUrl(champions, Number(champion.key))} alt="" />
      <span className="directory-card-name">{champion.name}</span>
      <span className="directory-card-title">{champion.title ?? 'Champion guide'}</span>
    </button>
  );
}

function DirectorySummaryStat({ label, value }: { label: string; value: string }) {
  return <MetricTile label={label} value={value} />;
}

function sortChampions(a: Champion, b: Champion, sortMode: SortMode) {
  if (sortMode === 'name-desc') {
    return b.name.localeCompare(a.name);
  }
  if (sortMode === 'title-asc') {
    return (a.title ?? '').localeCompare(b.title ?? '') || a.name.localeCompare(b.name);
  }
  return a.name.localeCompare(b.name);
}

function formatNumber(value: number) {
  return new Intl.NumberFormat('en-US').format(value);
}
