import { Database, Filter, Search } from 'lucide-react';
import { useMemo, useState } from 'react';
import type { CSSProperties } from 'react';
import { useQuery } from '@tanstack/react-query';
import { getChampionGuideIndex } from '../api/client';
import type { Champion, ChampionData, ChampionGuideSummary } from '../api/types';
import { normalizeLookup } from '../lib/lookup';
import { ROLE_OPTIONS_WITH_ALL, RoleIcon, roleLabel } from '../lib/roles';
import { championByKey, championSplashUrl } from '../lib/staticData';
import { championTier } from '../lib/tiers';
import { ChampionIdentity } from './ui/ChampionIdentity';
import { MetricTile } from './ui/MetricTile';
import { RoleTabs } from './ui/RoleTabs';
import { SelectControl } from './ui/SelectControl';

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
  { value: 'confidence', label: 'WinRift Score' },
  { value: 'win-rate', label: 'Win Rate' },
  { value: 'impact', label: 'Impact' },
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
          tier: championTier(summary),
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
            A multi-signal read on champion strength from collected ranked Solo/Duo games: winrate, confidence, sample size, pick/ban pressure, and role-relative match impact.
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
        <RoleTabs
          ariaLabel="Tier list role"
          className="guide-role-tabs tier-role-tabs"
          options={ROLE_OPTIONS_WITH_ALL}
          value={role}
          onChange={setRole}
        />
        <SelectControl className="guide-select-control compact" label="Rank" value={rankBucket} onChange={setRankBucket}>
          {ranks.map((candidate) => (
            <option key={candidate.value || 'ALL'} value={candidate.value}>{candidate.label}</option>
          ))}
        </SelectControl>
        <SelectControl className="guide-select-control compact" label="Sample" value={minGames} onChange={(value) => setMinGames(Number(value))}>
          {minimumSamples.map((candidate) => (
            <option key={candidate.value} value={candidate.value}>{candidate.label}</option>
          ))}
        </SelectControl>
        <SelectControl className="guide-select-control compact" label="Sort" value={sortMode} onChange={(value) => setSortMode(value as SortMode)}>
          {sortModes.map((candidate) => (
            <option key={candidate.value} value={candidate.value}>{candidate.label}</option>
          ))}
        </SelectControl>
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
          <span>Impact</span>
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
  return <MetricTile label={label} value={value} />;
}

function TierFeatureCard({ row, champions, onSelectChampion }: { row: TierRow; champions?: ChampionData; onSelectChampion: (champion: Champion) => void }) {
  const champion = row.champion;
  return (
    <button
      className={`tier-feature-card ${tierClassName(row.tier)}`}
      disabled={!champion}
      onClick={() => champion && onSelectChampion(champion)}
      style={{ '--tier-splash': `url(${championSplashUrl(champions, row.summary.championId)})` } as CSSProperties}
      type="button"
    >
      <span className={`tier-badge ${tierClassName(row.tier)}`}>{row.tier}</span>
      <ChampionIdentity
        as="div"
        champion={champion}
        championId={row.summary.championId}
        champions={champions}
        className="tier-feature-identity"
        detail={<><RoleIcon role={row.summary.role} /> #{row.summary.roleRank || '-'} {roleLabel(row.summary.role)}</>}
      />
      <p>{row.summary.winRate.toFixed(2)}% WR · {formatNumber(row.summary.games)} games · {row.score.toFixed(1)} score · {impactLabel(row.summary)}</p>
    </button>
  );
}

function TierTableRow({ row, champions, onSelectChampion }: { row: TierRow; champions?: ChampionData; onSelectChampion: (champion: Champion) => void }) {
  const champion = row.champion;
  return (
    <button className="tier-table-row" disabled={!champion} onClick={() => champion && onSelectChampion(champion)} type="button">
      <span><b className={`tier-badge ${tierClassName(row.tier)}`}>{row.tier}</b></span>
      <ChampionIdentity
        champion={champion}
        championId={row.summary.championId}
        champions={champions}
        className="tier-champion-cell"
        detail={<><RoleIcon role={row.summary.role} /> Rank {row.summary.roleRank || '-'} of {row.summary.roleRankTotal || '-'}</>}
      />
      <span>{row.summary.winRate.toFixed(2)}%</span>
      <span>{impactLabel(row.summary)}</span>
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
  return summary.tierScore ?? summary.confidence ?? 0;
}

function tierClassName(tier: string) {
  if (tier === 'S+') return 'tier-s-plus';
  return `tier-${tier.toLowerCase().replace(/[^a-z0-9]+/g, '-') || 'unknown'}`;
}

function impactLabel(summary: ChampionGuideSummary) {
  if (typeof summary.impactScore === 'number' && summary.impactScore > 0) {
    return `${summary.impactScore.toFixed(1)} impact`;
  }
  if (typeof summary.kda === 'number' && summary.kda > 0) {
    return `${summary.kda.toFixed(2)} KDA`;
  }
  return '--';
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
  if (sortMode === 'impact') {
    return (b.summary.impactScore ?? 0) - (a.summary.impactScore ?? 0) || b.score - a.score;
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
