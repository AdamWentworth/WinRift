import { Database, Filter, Search, Trophy } from 'lucide-react';
import { useMemo, useState } from 'react';
import type { CSSProperties } from 'react';
import { useQuery } from '@tanstack/react-query';
import { getChampionGuideIndex } from '../api/client';
import type { Champion, ChampionData, ChampionGuideSummary } from '../api/types';
import { normalizeLookup } from '../lib/lookup';
import { ROLE_OPTIONS_WITH_ALL, RoleIcon, roleLabel } from '../lib/roles';
import { championByKey, championImageUrl, championSplashUrl } from '../lib/staticData';

const ranks = [
  { value: '', label: 'All Ranks' },
  { value: 'MASTER+', label: 'Master+' },
  { value: 'DIAMOND', label: 'Diamond' },
  { value: 'EMERALD', label: 'Emerald' },
  { value: 'PLATINUM', label: 'Platinum' },
  { value: 'GOLD', label: 'Gold' },
];

const minimumSamples = [
  { value: 20, label: '20+ games' },
  { value: 50, label: '50+ games' },
  { value: 100, label: '100+ games' },
  { value: 250, label: '250+ games' },
];

const sortModes = [
  { value: 'rank', label: 'WinRift Rank' },
  { value: 'confidence', label: 'Score' },
  { value: 'win-rate', label: 'Win Rate' },
  { value: 'pick-rate', label: 'Pick Rate' },
  { value: 'ban-rate', label: 'Ban Rate' },
  { value: 'games', label: 'Games' },
  { value: 'name', label: 'Name' },
];

type SortMode = typeof sortModes[number]['value'];

type Props = {
  champions?: ChampionData;
  onSelectChampion: (champion: Champion) => void;
};

type TierRow = {
  champion?: Champion;
  score: number;
  summary: ChampionGuideSummary;
  tier: string;
};

export function TierListPage({ champions, onSelectChampion }: Props) {
  const [role, setRole] = useState('');
  const [rankBucket, setRankBucket] = useState('');
  const [minGames, setMinGames] = useState(50);
  const [sortMode, setSortMode] = useState<SortMode>('rank');
  const [searchText, setSearchText] = useState('');
  const patch = patchBucket(champions?.version);
  const tierQuery = useQuery({
    queryKey: ['tier-list', role, patch, rankBucket, minGames],
    queryFn: () => getChampionGuideIndex({ role, patch, rankBucket, minGames, limit: 300 }),
    enabled: Boolean(patch),
    staleTime: 5 * 60 * 1000,
  });
  const selectedRole = roleLabel(role);
  const selectedRank = ranks.find((candidate) => candidate.value === rankBucket)?.label ?? 'All Ranks';
  const normalizedSearch = normalizeLookup(searchText);
  const rows = useMemo(() => {
    return (tierQuery.data?.results ?? [])
      .map((summary) => {
        const champion = championByKey(champions, summary.championId);
        return {
          champion,
          score: winriftScore(summary),
          summary,
          tier: tierForSummary(summary),
        };
      })
      .filter((row) => {
        if (!normalizedSearch) return true;
        return normalizeLookup(row.champion?.name ?? String(row.summary.championId)).includes(normalizedSearch);
      })
      .sort((a, b) => sortRows(a, b, sortMode));
  }, [champions, normalizedSearch, sortMode, tierQuery.data?.results]);
  const featured = rows.slice(0, 3);
  const totalGames = (tierQuery.data?.results ?? []).reduce((sum, row) => sum + row.games, 0);

  return (
    <section className="tier-list-page">
      <div className="tier-list-hero">
        <div className="tier-list-hero-copy">
          <span>WinRift Tier List</span>
          <h2>{selectedRole} Rankings</h2>
          <p>
            A confidence-weighted read on champion strength from collected ranked Solo/Duo games. Use this as the broad meta view, then open a champion guide for builds, runes, skill paths, and matchup detail.
          </p>
        </div>
        <div className="tier-list-hero-stats">
          <TierHeroStat label="Patch" value={patch || champions?.version || 'Current'} />
          <TierHeroStat label="Rank" value={selectedRank} />
          <TierHeroStat label="Matches Indexed" value={formatNumber(totalGames)} />
        </div>
      </div>

      <div className="tier-filter-bar">
        <div className="guide-filter-label">
          <Filter size={17} />
          <span>Filters</span>
        </div>
        <div className="guide-role-tabs tier-role-tabs" aria-label="Tier list role">
          {ROLE_OPTIONS_WITH_ALL.map((candidate) => (
            <button key={candidate.value || 'ALL'} className={candidate.value === role ? 'selected' : ''} onClick={() => setRole(candidate.value)} type="button">
              <RoleIcon role={candidate.value} />
              <span>{candidate.label}</span>
            </button>
          ))}
        </div>
        <label className="guide-select-control compact">
          <span>Rank</span>
          <select value={rankBucket} onChange={(event) => setRankBucket(event.target.value)}>
            {ranks.map((candidate) => (
              <option key={candidate.value || 'ALL'} value={candidate.value}>{candidate.label}</option>
            ))}
          </select>
        </label>
        <label className="guide-select-control compact">
          <span>Sample</span>
          <select value={minGames} onChange={(event) => setMinGames(Number(event.target.value))}>
            {minimumSamples.map((candidate) => (
              <option key={candidate.value} value={candidate.value}>{candidate.label}</option>
            ))}
          </select>
        </label>
        <label className="guide-select-control compact">
          <span>Sort</span>
          <select value={sortMode} onChange={(event) => setSortMode(event.target.value as SortMode)}>
            {sortModes.map((candidate) => (
              <option key={candidate.value} value={candidate.value}>{candidate.label}</option>
            ))}
          </select>
        </label>
      </div>

      <div className="tier-list-tools">
        <label className="champion-directory-search tier-search">
          <Search size={16} />
          <input value={searchText} onChange={(event) => setSearchText(event.target.value)} placeholder="Search champions in this tier list" />
        </label>
        <div className="tier-list-scope">
          <Database size={15} />
          <span>{tierQuery.isLoading ? 'Loading rankings...' : `${formatNumber(rows.length)} champions shown`}</span>
          <b>{selectedRole} / {selectedRank} / {minimumSamples.find((sample) => sample.value === minGames)?.label}</b>
        </div>
      </div>

      <div className="tier-feature-grid">
        {featured.length ? featured.map((row) => (
          <TierFeatureCard key={row.summary.championId} row={row} champions={champions} onSelectChampion={onSelectChampion} />
        )) : (
          <div className="tier-empty-state">
            {tierQuery.isLoading ? 'Loading top champions...' : 'No champions meet this sample yet.'}
          </div>
        )}
      </div>

      <div className="tier-table-card">
        <div className="tier-table-header">
          <span>Tier</span>
          <span>Champion</span>
          <span>Win Rate</span>
          <span>Pick</span>
          <span>Ban</span>
          <span>Score</span>
          <span>Games</span>
        </div>
        <div className="tier-table-body">
          {rows.length ? rows.map((row) => (
            <TierTableRow key={`${row.summary.championId}-${row.summary.role}`} row={row} champions={champions} onSelectChampion={onSelectChampion} />
          )) : (
            <div className="tier-empty-row">{tierQuery.isLoading ? 'Loading champion rows...' : 'No tier-list rows for this filter yet.'}</div>
          )}
        </div>
      </div>
    </section>
  );
}

function TierHeroStat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function TierFeatureCard({ row, champions, onSelectChampion }: { row: TierRow; champions?: ChampionData; onSelectChampion: (champion: Champion) => void }) {
  const champion = row.champion;
  return (
    <button
      className={`tier-feature-card ${row.tier}`}
      disabled={!champion}
      onClick={() => champion && onSelectChampion(champion)}
      style={{ '--tier-splash': `url(${championSplashUrl(champions, row.summary.championId)})` } as CSSProperties}
      type="button"
    >
      <span className={`tier-badge ${row.tier}`}>{row.tier}</span>
      <div>
        {champion ? <img src={championImageUrl(champions, row.summary.championId)} alt="" /> : null}
        <span>
          <strong>{champion?.name ?? row.summary.championId}</strong>
          <em><RoleIcon role={row.summary.role} /> #{row.summary.roleRank || '-'} {roleLabel(row.summary.role)}</em>
        </span>
      </div>
      <p>{row.summary.winRate.toFixed(2)}% WR · {formatNumber(row.summary.games)} games · {row.score.toFixed(1)} score</p>
    </button>
  );
}

function TierTableRow({ row, champions, onSelectChampion }: { row: TierRow; champions?: ChampionData; onSelectChampion: (champion: Champion) => void }) {
  const champion = row.champion;
  return (
    <button className="tier-table-row" disabled={!champion} onClick={() => champion && onSelectChampion(champion)} type="button">
      <span><b className={`tier-badge ${row.tier}`}>{row.tier}</b></span>
      <span className="tier-champion-cell">
        {champion ? <img src={championImageUrl(champions, row.summary.championId)} alt="" /> : null}
        <span>
          <strong>{champion?.name ?? row.summary.championId}</strong>
          <em><RoleIcon role={row.summary.role} /> Rank {row.summary.roleRank || '-'} of {row.summary.roleRankTotal || '-'}</em>
        </span>
      </span>
      <span>{row.summary.winRate.toFixed(2)}%</span>
      <span>{row.summary.pickRate.toFixed(2)}%</span>
      <span>{row.summary.banRate.toFixed(2)}%</span>
      <span>{row.score.toFixed(1)}</span>
      <span>{formatNumber(row.summary.games)}</span>
    </button>
  );
}

function patchBucket(version?: string) {
  const parts = (version ?? '').split('.');
  if (parts.length >= 2) {
    return `${parts[0]}.${parts[1]}`;
  }
  return '';
}

function winriftScore(summary: ChampionGuideSummary) {
  return summary.confidence || 0;
}

function tierForSummary(summary: ChampionGuideSummary) {
  if (!summary.roleRank || !summary.roleRankTotal) return '?';
  const percentile = summary.roleRank / summary.roleRankTotal;
  if (percentile <= 0.08) return 'S';
  if (percentile <= 0.22) return 'A';
  if (percentile <= 0.5) return 'B';
  if (percentile <= 0.75) return 'C';
  return 'D';
}

function sortRows(a: TierRow, b: TierRow, sortMode: SortMode) {
  if (sortMode === 'name') {
    return (a.champion?.name ?? '').localeCompare(b.champion?.name ?? '');
  }
  if (sortMode === 'confidence') {
    return b.score - a.score || a.summary.roleRank - b.summary.roleRank;
  }
  if (sortMode === 'win-rate') {
    return b.summary.winRate - a.summary.winRate || b.summary.games - a.summary.games;
  }
  if (sortMode === 'pick-rate') {
    return b.summary.pickRate - a.summary.pickRate || b.summary.games - a.summary.games;
  }
  if (sortMode === 'ban-rate') {
    return b.summary.banRate - a.summary.banRate || b.summary.games - a.summary.games;
  }
  if (sortMode === 'games') {
    return b.summary.games - a.summary.games || b.score - a.score;
  }
  return a.summary.roleRank - b.summary.roleRank;
}

function formatNumber(value: number) {
  return new Intl.NumberFormat('en-US').format(value);
}
