import { Search } from 'lucide-react';
import type { BuildFilters, ChampionData } from '../api/types';
import { championList } from '../lib/staticData';

const roles = ['TOP', 'JUNGLE', 'MIDDLE', 'BOTTOM', 'UTILITY'];
const ranks = ['UNKNOWN', 'IRON', 'BRONZE', 'SILVER', 'GOLD', 'PLATINUM', 'EMERALD', 'DIAMOND', 'MASTER+'];

type Props = {
  champions?: ChampionData;
  filters: BuildFilters;
  onChange: (filters: BuildFilters) => void;
  onSubmit: () => void;
};

export function BuildExplorer({ champions, filters, onChange, onSubmit }: Props) {
  const options = championList(champions);

  return (
    <section className="panel">
      <div className="panel-heading">
        <h2>Build Explorer</h2>
        <button className="icon-button primary" onClick={onSubmit} title="Search build patterns" aria-label="Search build patterns">
          <Search size={18} />
        </button>
      </div>
      <div className="filter-grid">
        <label>
          Champion
          <select value={filters.championId ?? ''} onChange={(event) => onChange({ ...filters, championId: Number(event.target.value) || undefined })}>
            <option value="">Any champion</option>
            {options.map((champion) => (
              <option key={champion.key} value={champion.key}>{champion.name}</option>
            ))}
          </select>
        </label>
        <label>
          Role
          <select value={filters.role ?? ''} onChange={(event) => onChange({ ...filters, role: event.target.value || undefined })}>
            <option value="">Any role</option>
            {roles.map((role) => (
              <option key={role} value={role}>{role}</option>
            ))}
          </select>
        </label>
        <label>
          Opponent
          <select value={filters.opponentChampionId ?? ''} onChange={(event) => onChange({ ...filters, opponentChampionId: Number(event.target.value) || undefined })}>
            <option value="">Any opponent</option>
            {options.map((champion) => (
              <option key={champion.key} value={champion.key}>{champion.name}</option>
            ))}
          </select>
        </label>
        <label>
          Patch
          <input value={filters.patch ?? ''} placeholder="16.10" onChange={(event) => onChange({ ...filters, patch: event.target.value || undefined })} />
        </label>
        <label>
          Rank
          <select value={filters.rankBucket ?? ''} onChange={(event) => onChange({ ...filters, rankBucket: event.target.value || undefined })}>
            <option value="">Any rank</option>
            {ranks.map((rank) => (
              <option key={rank} value={rank}>{rank}</option>
            ))}
          </select>
        </label>
        <label>
          Min Games
          <input type="number" min={1} value={filters.minGames} onChange={(event) => onChange({ ...filters, minGames: Number(event.target.value) || 1 })} />
        </label>
      </div>
    </section>
  );
}
