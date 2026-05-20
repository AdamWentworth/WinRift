import { Search, Trophy } from 'lucide-react';
import { useMemo, useState } from 'react';
import type { CSSProperties } from 'react';
import { useQuery } from '@tanstack/react-query';
import { getChampionGuideIndex } from '../api/client';
import type { Champion, ChampionData, ChampionGuideSummary } from '../api/types';
import { championImageUrl, championList, championSplashUrl } from '../lib/staticData';
import { normalizeLookup } from '../lib/lookup';

const roles = [
  { value: 'TOP', label: 'Top' },
  { value: 'JUNGLE', label: 'Jungle' },
  { value: 'MIDDLE', label: 'Mid' },
  { value: 'BOTTOM', label: 'Bot' },
  { value: 'UTILITY', label: 'Support' },
];

const ranks = [
  { value: '', label: 'All Ranks' },
  { value: 'MASTER+', label: 'Master+' },
  { value: 'DIAMOND', label: 'Diamond' },
  { value: 'EMERALD', label: 'Emerald' },
  { value: 'PLATINUM', label: 'Platinum' },
  { value: 'GOLD', label: 'Gold' },
];

type Props = {
  champions?: ChampionData;
  onSelectChampion: (champion: Champion) => void;
};

export function ChampionDirectoryPage({ champions, onSelectChampion }: Props) {
  const [role, setRole] = useState('JUNGLE');
  const [rankBucket, setRankBucket] = useState('');
  const [searchText, setSearchText] = useState('');
  const patch = patchBucket(champions?.version);
  const championsByName = useMemo(() => championList(champions), [champions]);
  const guideIndexQuery = useQuery({
    queryKey: ['champion-directory-index', role, patch, rankBucket],
    queryFn: () => getChampionGuideIndex({ role, patch, rankBucket, minGames: 1, limit: 300 }),
    enabled: Boolean(patch),
    staleTime: 5 * 60 * 1000,
  });
  const coverageByChampionId = useMemo(() => {
    const map = new Map<number, ChampionGuideSummary>();
    for (const summary of guideIndexQuery.data?.results ?? []) {
      map.set(summary.championId, summary);
    }
    return map;
  }, [guideIndexQuery.data?.results]);
  const normalizedSearch = normalizeLookup(searchText);
  const filteredChampions = useMemo(() => {
    return championsByName
      .filter((champion) => {
        if (!normalizedSearch) return true;
        return normalizeLookup(champion.name).includes(normalizedSearch) || normalizeLookup(champion.id).includes(normalizedSearch);
      })
      .sort((a, b) => {
        const aSummary = coverageByChampionId.get(Number(a.key));
        const bSummary = coverageByChampionId.get(Number(b.key));
        if (aSummary && bSummary && aSummary.roleRank !== bSummary.roleRank) return aSummary.roleRank - bSummary.roleRank;
        if (aSummary && !bSummary) return -1;
        if (!aSummary && bSummary) return 1;
        return a.name.localeCompare(b.name);
      });
  }, [championsByName, coverageByChampionId, normalizedSearch]);
  const totalGames = (guideIndexQuery.data?.results ?? []).reduce((sum, summary) => sum + summary.games, 0);
  const selectedRole = roles.find((candidate) => candidate.value === role)?.label ?? role;
  const selectedRank = ranks.find((candidate) => candidate.value === rankBucket)?.label ?? 'All Ranks';

  return (
    <section className="champion-directory-page">
      <div className="champion-directory-hero">
        <div>
          <span>Champion Index</span>
          <h2>Build Guides By Champion</h2>
          <p>Browse every champion, filter by role and rank, then open the collected-match guide with runes, spells, skill paths, item slots, and matchup context.</p>
        </div>
        <div className="champion-directory-summary">
          <DirectorySummaryStat label="Patch" value={patch || champions?.version || 'Current'} />
          <DirectorySummaryStat label="Role" value={selectedRole} />
          <DirectorySummaryStat label="Sample" value={`${formatNumber(totalGames)} games`} />
        </div>
      </div>

      <div className="champion-directory-toolbar">
        <label className="champion-directory-search">
          <Search size={16} />
          <input
            value={searchText}
            onChange={(event) => setSearchText(event.target.value)}
            placeholder="Search champions"
          />
        </label>
        <div className="guide-role-tabs" aria-label="Champion directory role">
          {roles.map((candidate) => (
            <button key={candidate.value} className={candidate.value === role ? 'selected' : ''} onClick={() => setRole(candidate.value)} type="button">
              {candidate.label}
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
      </div>

      <div className="champion-directory-meta">
        <span>{guideIndexQuery.isLoading ? 'Loading collected guide coverage...' : `${formatNumber(filteredChampions.length)} champions shown`}</span>
        <b>{selectedRole} / {selectedRank}</b>
      </div>

      <div className="champion-directory-grid">
        {filteredChampions.map((champion) => (
          <ChampionDirectoryCard
            key={champion.key}
            champion={champion}
            champions={champions}
            summary={coverageByChampionId.get(Number(champion.key))}
            onSelect={() => onSelectChampion(champion)}
          />
        ))}
      </div>
    </section>
  );
}

function ChampionDirectoryCard({ champion, champions, summary, onSelect }: { champion: Champion; champions?: ChampionData; summary?: ChampionGuideSummary; onSelect: () => void }) {
  return (
    <button
      className="champion-directory-card"
      onClick={onSelect}
      style={{ '--champion-splash': `url(${championSplashUrl(champions, Number(champion.key))})` } as CSSProperties}
      type="button"
    >
      <span className={`directory-tier ${guideTier(summary)}`}>{guideTier(summary)}</span>
      <img src={championImageUrl(champions, Number(champion.key))} alt="" />
      <span className="directory-card-name">{champion.name}</span>
      <span className="directory-card-title">{champion.title ?? 'Champion guide'}</span>
      <span className="directory-card-stats">
        <DirectoryCardStat label="WR" value={summary?.games ? `${summary.winRate.toFixed(1)}%` : '--'} />
        <DirectoryCardStat label="Pick" value={summary?.games ? `${summary.pickRate.toFixed(1)}%` : '--'} />
        <DirectoryCardStat label="Ban" value={summary?.games ? `${summary.banRate.toFixed(1)}%` : '--'} />
        <DirectoryCardStat label="Games" value={formatNumber(summary?.games ?? 0)} />
      </span>
      {summary?.roleRank ? (
        <span className="directory-rank-chip">
          <Trophy size={12} />
          {summary.roleRank}/{summary.roleRankTotal}
        </span>
      ) : null}
    </button>
  );
}

function DirectorySummaryStat({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function DirectoryCardStat({ label, value }: { label: string; value: string }) {
  return (
    <span>
      <em>{label}</em>
      <strong>{value}</strong>
    </span>
  );
}

function patchBucket(version?: string) {
  const parts = (version ?? '').split('.');
  if (parts.length >= 2) {
    return `${parts[0]}.${parts[1]}`;
  }
  return '';
}

function guideTier(summary?: ChampionGuideSummary) {
  if (!summary?.games) return '?';
  if (summary.games >= 250 && summary.winRate >= 53) return 'S';
  if (summary.games >= 100 && summary.winRate >= 51.5) return 'A';
  if (summary.winRate >= 50) return 'B';
  if (summary.winRate >= 48) return 'C';
  return 'D';
}

function formatNumber(value: number) {
  return new Intl.NumberFormat('en-US').format(value);
}
