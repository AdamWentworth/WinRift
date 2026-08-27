import { Database, Filter, Search, Shield } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import type { CSSProperties } from 'react';
import { useQuery } from '@tanstack/react-query';
import { getChampionPageBundle } from '../api/client';
import type { AnalyticsPatchStat, BuildAdviceResponse, Champion, ChampionData, ChampionGuideBuildVariant, ChampionGuideResponse, ChampionGuideSummary, ChampionRoleRate, ItemData, RuneData, SummonerSpellData } from '../api/types';
import {
  championAbilityImageUrl,
  championByKey,
  championImageUrl,
  championList,
  championSplashUrl,
} from '../lib/staticData';
import { itemContextForRole, mainChampionRole } from '../lib/championRoles';
import { patchBucketFromVersion } from '../lib/patches';
import { queryStaleTime } from '../lib/queryPolicies';
import { CHAMPION_PAGE_QUERY_VERSION } from '../lib/queryVersions';
import { ROLE_OPTIONS, RoleIcon, roleLabel } from '../lib/roles';
import { championTier } from '../lib/tiers';
import { MetricTile } from './ui/MetricTile';
import { PanelCard, PanelTitle } from './ui/Panel';
import { PatchScopeControl } from './ui/PatchScopeControl';
import { RoleTabs } from './ui/RoleTabs';
import { SelectControl } from './ui/SelectControl';
import { BuildVariantTabs, buildVariantLabel, groupBuildVariantsForDisplay, RECOMMENDED_BUILD_KEY } from './build-guide/BuildVariantTabs';
import { MatchupStrip, RuneGuideCard, SkillGuideCard, SkillPathCard, SpellGuideCard } from './build-guide/GuideDataCards';
import { ItemGuideGrid, type GuideItemSlot, type GuideStartingLoadout } from './build-guide/GuideItemPanels';

export type { GuideItemSlot, GuideStartingLoadout } from './build-guide/GuideItemPanels';
export { selectGuideItemPanelRows, selectStartingLoadoutRows } from './build-guide/GuideItemPanels';

const ranks = [
  { value: '', label: 'All Ranks' },
  { value: 'MASTER+', label: 'Master+' },
  { value: 'DIAMOND', label: 'Diamond' },
  { value: 'EMERALD', label: 'Emerald' },
  { value: 'PLATINUM', label: 'Platinum' },
  { value: 'GOLD', label: 'Gold' },
];

const DEFAULT_QUEUE_ID = 420;

type Props = {
  champions?: ChampionData;
  items?: ItemData;
  spells?: SummonerSpellData;
  runes?: RuneData;
  initialChampionId?: number;
  analyticsPatch?: string;
  analyticsFallbackPatch?: string;
  analyticsPatchLoading?: boolean;
  analyticsPatchOptions?: AnalyticsPatchStat[];
  currentAnalyticsPatch?: string;
  roleRates?: ChampionRoleRate[];
  onAnalyticsPatchChange?: (patch: string) => void;
  onChampionChange?: (champion: Champion) => void;
};

export function BuildGuidePage({
  champions,
  items,
  spells,
  runes,
  initialChampionId,
  analyticsPatch,
  analyticsFallbackPatch = '',
  analyticsPatchLoading = false,
  analyticsPatchOptions = [],
  currentAnalyticsPatch = '',
  roleRates,
  onAnalyticsPatchChange,
  onChampionChange,
}: Props) {
  const championsByName = useMemo(() => championList(champions), [champions]);
  const defaultChampionId = useMemo(() => {
    const wukong = championsByName.find((champion) => champion.id === 'MonkeyKing');
    return Number(wukong?.key ?? championsByName[0]?.key ?? 0);
  }, [championsByName]);
  const [championId, setChampionId] = useState(initialChampionId ?? defaultChampionId);
  const [role, setRole] = useState('');
  const [roleTouched, setRoleTouched] = useState(false);
  const [rankBucket, setRankBucket] = useState('');
  const [opponentChampionId, setOpponentChampionId] = useState(0);
  const [selectedBuildVariantKey, setSelectedBuildVariantKey] = useState('');
  const patch = analyticsPatch ?? patchBucketFromVersion(champions?.version);
  const champion = championByKey(champions, championId);
  const opponent = opponentChampionId ? championByKey(champions, opponentChampionId) : undefined;
  const seededRoleRates = useMemo(() => (
    roleRates?.filter((row) => row.championId === championId) ?? []
  ), [championId, roleRates]);
  const queryRole = roleTouched ? role : '';
  const itemContext = roleTouched ? itemContextForRole(role) : undefined;
  const championPageQuery = useQuery({
    queryKey: ['champion-page', CHAMPION_PAGE_QUERY_VERSION, championId, queryRole || 'AUTO', patch, rankBucket, opponentChampionId],
    queryFn: ({ signal }) => getChampionPageBundle({
      championId,
      role: queryRole || undefined,
      itemContext,
      opponentChampionId: opponentChampionId || undefined,
      patch,
      rankBucket,
      minGames: opponentChampionId ? 3 : 5,
      championMinGames: 10,
      guideMinGames: 5,
      guideLimit: 12,
      indexMinGames: 1,
      indexLimit: 250,
      queueId: DEFAULT_QUEUE_ID,
      limit: 4,
    }, { signal }),
    enabled: Boolean(championId && patch && (!roleTouched || role)),
    staleTime: queryStaleTime.championGuide,
  });
  const primaryBundle = championPageQuery.data;
  const fallbackRole = queryRole;
  const shouldLoadFallback = Boolean(
    primaryBundle
    && primaryBundle.guide.summary.games <= 0
    && analyticsFallbackPatch
    && analyticsFallbackPatch !== patch,
  );
  const fallbackPageQuery = useQuery({
    queryKey: ['champion-page', CHAMPION_PAGE_QUERY_VERSION, championId, fallbackRole || 'AUTO', analyticsFallbackPatch, rankBucket, opponentChampionId],
    queryFn: ({ signal }) => getChampionPageBundle({
      championId,
      role: fallbackRole || undefined,
      itemContext: fallbackRole ? itemContextForRole(fallbackRole) : undefined,
      opponentChampionId: opponentChampionId || undefined,
      patch: analyticsFallbackPatch,
      rankBucket,
      minGames: opponentChampionId ? 3 : 5,
      championMinGames: 10,
      guideMinGames: 5,
      guideLimit: 12,
      indexMinGames: 1,
      indexLimit: 250,
      queueId: DEFAULT_QUEUE_ID,
      limit: 4,
    }, { signal }),
    enabled: Boolean(championId && shouldLoadFallback),
    staleTime: queryStaleTime.championGuide,
  });
  const fallbackInUse = Boolean(fallbackPageQuery.data && fallbackPageQuery.data.guide.summary.games > 0);
  const pageBundle = fallbackInUse ? fallbackPageQuery.data : primaryBundle;
  const effectiveDataPatch = fallbackInUse ? analyticsFallbackPatch : patch;
  const bundleRoleRates = pageBundle?.roleRates.results ?? [];
  const activeRoleRates = seededRoleRates.length ? seededRoleRates : bundleRoleRates;
  const mainRole = useMemo(() => mainChampionRole(activeRoleRates, championId), [activeRoleRates, championId]);
  const resolvedRole = pageBundle?.filters.role || mainRole || '';
  const displayRole = roleTouched ? role : resolvedRole;
  const waitingForFallback = shouldLoadFallback && fallbackPageQuery.isLoading && !fallbackPageQuery.data;
  const visibleLoading = useDelayedFlag((championPageQuery.isLoading && !primaryBundle) || waitingForFallback, 180);
  const showGuideData = Boolean(pageBundle) || visibleLoading;

  useEffect(() => {
    if (initialChampionId && championByKey(champions, initialChampionId) && initialChampionId !== championId) {
      setRoleTouched(false);
      setRole('');
      setChampionId(initialChampionId);
      return;
    }
    if (defaultChampionId && !championByKey(champions, championId)) {
      setRoleTouched(false);
      setRole('');
      setChampionId(defaultChampionId);
    }
  }, [champions, championId, defaultChampionId, initialChampionId]);

  useEffect(() => {
    setRoleTouched(false);
    setRole('');
  }, [championId]);

  useEffect(() => {
    setSelectedBuildVariantKey('');
  }, [championId, effectiveDataPatch, queryRole, rankBucket]);

  const updateChampion = (value: number) => {
    setRoleTouched(false);
    setRole('');
    setChampionId(value);
    const nextChampion = championByKey(champions, value);
    if (nextChampion) {
      onChampionChange?.(nextChampion);
    }
  };
  const updateRole = (value: string) => {
    setRoleTouched(true);
    setRole(value);
  };

  const guide = pageBundle?.guide;
  const buildAdvice = pageBundle?.buildAdvice;
  const guideForBuilds = buildAdvice ? mergeGuideWithBuildAdvice(guide, buildAdvice) : guide;
  const buildVariants = useMemo(() => groupBuildVariantsForDisplay(guideForBuilds?.buildVariants ?? [], items), [guideForBuilds?.buildVariants, items]);
  const selectedBuildVariant = selectedBuildVariantKey && selectedBuildVariantKey !== RECOMMENDED_BUILD_KEY
    ? buildVariants.find((variant) => variant.variantKey === selectedBuildVariantKey)
    : undefined;
  const selectedBuildVariantIndex = selectedBuildVariant ? buildVariants.indexOf(selectedBuildVariant) : -1;
  const selectedBuildVariantLabel = selectedBuildVariant ? buildVariantLabel(selectedBuildVariant, selectedBuildVariantIndex, items) : '';
  const recommendedItemSlots: GuideItemSlot[] = buildAdvice?.matchup.available && buildAdvice.matchup.itemSlots.length
    ? buildAdvice.matchup.itemSlots
    : buildAdvice?.champion.itemSlots ?? [];
  const recommendedStartingLoadouts: GuideStartingLoadout[] = buildAdvice?.matchup.available && buildAdvice.matchup.startingLoadouts?.length
    ? buildAdvice.matchup.startingLoadouts
    : buildAdvice?.champion.startingLoadouts ?? [];
  const selectedVariantItemSlots = selectedBuildVariant ? variantItemSlots(selectedBuildVariant) : [];
  const itemSlots: GuideItemSlot[] = selectedVariantItemSlots.length
    ? withStartingItemRows(recommendedItemSlots, selectedVariantItemSlots)
    : recommendedItemSlots;
  const itemSlotContext = selectedVariantItemSlots.length
    ? `${selectedBuildVariantLabel} selected build family`
    : buildAdviceContext(buildAdvice, opponent?.name);
  const guideIndex = pageBundle?.guideIndex.results ?? [];
  const coverageByChampionId = useMemo(() => {
    const map = new Map<number, ChampionGuideSummary>();
    for (const summary of guideIndex) {
      map.set(summary.championId, summary);
    }
    return map;
  }, [guideIndex]);
  const currentCoverage = coverageByChampionId.get(championId);
  const coveredGames = guideIndex.reduce((total, summary) => total + summary.games, 0);
  const splash = championSplashUrl(champions, championId);
  const rankLabel = ranks.find((candidate) => candidate.value === rankBucket)?.label ?? 'All Ranks';
  const titleRole = displayRole ? roleLabel(displayRole) : 'Role';
  const heroGamesLabel = pageBundle
    ? `${formatNumber(currentCoverage?.games ?? 0)} role games`
    : visibleLoading ? 'Loading role sample' : '\u00a0';
  const sampleContext = opponent ? `${champion?.name ?? championId} vs ${opponent.name}` : `${champion?.name ?? championId} overall`;

  return (
    <section className="build-guide-page">
      <div className="guide-hero" style={{ '--guide-splash': splash ? `url(${splash})` : undefined } as CSSProperties}>
        <div className="guide-hero-main">
          <div className="guide-champion-card">
            <span className={`guide-tier ${guideTierClassName(guideTier(guide))}`}>{guideTier(guide)}</span>
            {champion ? <img src={championImageUrl(champions, championId)} alt={champion.name} /> : null}
          </div>
          <div className="guide-title-block">
            <span className="guide-kicker">WinRift Build Atlas</span>
            <h2>
              <span>{champion?.name ?? 'Champion'}</span> <em><RoleIcon role={displayRole} /> {titleRole} patterns</em>
            </h2>
            <div className="guide-ability-row">
              <AbilityIcon champions={champions} ability={champion?.passive} label="P" />
              {(champion?.spells ?? []).slice(0, 4).map((ability, index) => (
              <AbilityIcon key={ability.id} champions={champions} ability={ability} label={['Q', 'W', 'E', 'R'][index]} folder="spell" />
              ))}
            </div>
            <p>
              A collected-match readout for ranked Solo/Duo: matchup-aware items, rune patterns, spell pairs, and where the sample is still thin.
            </p>
          </div>
        </div>
        <div className="guide-hero-aside">
          <PatchScopeControl
            activePatch={patch}
            className="guide-patch-scope"
            currentPatch={currentAnalyticsPatch || patch}
            loading={analyticsPatchLoading}
            options={analyticsPatchOptions}
            onChange={onAnalyticsPatchChange ?? (() => undefined)}
          />
          <div className="guide-hero-scope-card">
            <span>Rank Scope</span>
            <b>{rankLabel}</b>
            <em>{heroGamesLabel}</em>
          </div>
        </div>
      </div>

      <GuideFilters
        champions={championsByName}
        coverage={coverageByChampionId}
        championId={championId}
        role={displayRole}
        rankBucket={rankBucket}
        opponentChampionId={opponentChampionId}
        onChampionChange={updateChampion}
        onRoleChange={updateRole}
        onRankChange={setRankBucket}
        onOpponentChange={setOpponentChampionId}
      />

      {showGuideData ? (
        <>
          <GuideStats guide={guideForBuilds} loading={visibleLoading} />
          <GuideCoverage
            loading={visibleLoading}
            championCount={guideIndex.length}
            totalGames={coveredGames}
            selectedGames={currentCoverage?.games ?? 0}
            role={titleRole}
            rankLabel={rankLabel}
            patch={effectiveDataPatch}
          />

          <div className="guide-context-banner">
            <span>Current Sample</span>
            <b>{sampleContext}</b>
            <em>{fallbackInUse
              ? `${effectiveDataPatch} guide data while ${patch} fills`
              : opponent ? 'matchup-filtered where possible, then widened carefully' : 'champion-wide until a matchup is selected'}</em>
          </div>

          <BuildVariantTabs
            variants={buildVariants}
            recommendedGuide={guideForBuilds}
            recommendedItemSlots={recommendedItemSlots}
            selectedKey={selectedBuildVariant?.variantKey ?? RECOMMENDED_BUILD_KEY}
            items={items}
            onSelect={setSelectedBuildVariantKey}
          />

          <div className="guide-primary-grid">
            <RuneGuideCard guide={guideForBuilds} variant={selectedBuildVariant} runes={runes} loading={visibleLoading} />
            <div className="guide-side-stack">
              <SpellGuideCard guide={guideForBuilds} variant={selectedBuildVariant} spells={spells} loading={visibleLoading} />
              <GuideMiniNote
                title="Matchup Lens"
                body={buildAdvice?.notes[0] ?? (opponent ? `Item panels are narrowed to ${opponent.name} when the sample exists, then fall back to broader ${champion?.name ?? 'champion'} data.` : 'Choose a matchup filter to compare item choices into a specific opponent.')}
              />
            </div>
          </div>

          <div className="guide-skill-items-row build-linked">
            <SkillGuideCard guide={guide} variant={selectedBuildVariant} champion={champion} champions={champions} loading={visibleLoading} />
            <SkillPathCard guide={guide} variant={selectedBuildVariant} championName={champion?.name ?? 'this champion'} loading={visibleLoading} />
          </div>

          <BuildAdviceCoverage buildAdvice={buildAdvice} loading={visibleLoading} />

          <ItemGuideGrid rows={itemSlots} startingLoadouts={recommendedStartingLoadouts} items={items} loading={visibleLoading} context={itemSlotContext} />

          {displayRole === 'JUNGLE' ? <RoleQuestCard /> : null}

          <section className="guide-matchups-section" aria-label={`${champion?.name ?? 'Champion'} matchups`}>
            <div className="guide-section-title">
              <span>Matchups</span>
              <em>Counter picks and favorable opponents from stored ranked games.</em>
            </div>
            <div className="guide-matchups-grid">
              <MatchupStrip title="Toughest Matchups" subtitle={`These champions have performed best into ${champion?.name ?? 'this champion'}`} rows={guide?.toughestMatchups ?? []} champions={champions} tone="bad" loading={visibleLoading} />
              <MatchupStrip title="Favorable Matchups" subtitle={`${champion?.name ?? 'This champion'} has performed well into these opponents`} rows={guide?.bestMatchups ?? []} champions={champions} tone="good" loading={visibleLoading} />
            </div>
          </section>
        </>
      ) : (
        <div className="guide-fast-load-gap" aria-hidden="true" />
      )}
    </section>
  );
}

function useDelayedFlag(active: boolean, delayMs: number) {
  const [visible, setVisible] = useState(false);
  useEffect(() => {
    if (!active) {
      setVisible(false);
      return undefined;
    }
    const timer = window.setTimeout(() => setVisible(true), delayMs);
    return () => window.clearTimeout(timer);
  }, [active, delayMs]);
  return visible;
}

function BuildAdviceCoverage({ buildAdvice, loading }: { buildAdvice?: BuildAdviceResponse; loading: boolean }) {
  const matchupSample = buildAdvice?.matchup.sample;
  const championSample = buildAdvice?.champion.sample;
  return (
    <div className="guide-build-advice-strip">
      <div className="guide-build-advice-title">
        <Database size={15} />
        <span>Build Advice Source</span>
      </div>
      <MetricTile
        className="guide-build-advice-stat"
        label="Matchup sample"
        value={loading ? '...' : matchupSample?.sampleQualityLabel ?? 'No matchup'}
      />
      <MetricTile
        className="guide-build-advice-stat"
        label="Matchup max"
        value={loading ? '...' : formatNumber(matchupSample?.maxGames ?? 0)}
      />
      <MetricTile
        className="guide-build-advice-stat"
        label="Champion sample"
        value={loading ? '...' : championSample?.sampleQualityLabel ?? 'No sample'}
      />
      <MetricTile
        className="guide-build-advice-stat"
        label="Fallback"
        value={loading ? '...' : buildAdvice ? (buildAdvice.matchup.sample.fallbackUsed ? 'Mixed' : 'Exact') : '--'}
      />
    </div>
  );
}

function GuideFilters({
  champions,
  coverage,
  championId,
  role,
  rankBucket,
  opponentChampionId,
  onChampionChange,
  onRoleChange,
  onRankChange,
  onOpponentChange,
}: {
  champions: Champion[];
  coverage: Map<number, ChampionGuideSummary>;
  championId: number;
  role: string;
  rankBucket: string;
  opponentChampionId: number;
  onChampionChange: (value: number) => void;
  onRoleChange: (value: string) => void;
  onRankChange: (value: string) => void;
  onOpponentChange: (value: number) => void;
}) {
  return (
    <div className="guide-filter-bar">
      <div className="guide-filter-label">
        <Filter size={17} />
        <span>Filters</span>
      </div>
      <SelectControl label="Champion" value={championId} onChange={(value) => onChampionChange(Number(value))}>
        {champions.map((champion) => {
          const games = coverage.get(Number(champion.key))?.games ?? 0;
          return (
            <option key={champion.key} value={champion.key}>
              {champion.name}{games ? ` (${formatNumber(games)})` : ''}
            </option>
          );
        })}
      </SelectControl>
      <RoleTabs options={ROLE_OPTIONS} value={role} onChange={onRoleChange} />
      <SelectControl className="guide-select-control compact" label="Rank" value={rankBucket} onChange={onRankChange}>
        {ranks.map((candidate) => (
          <option key={candidate.value || 'ALL'} value={candidate.value}>{candidate.label}</option>
        ))}
      </SelectControl>
      <SelectControl className="guide-select-control matchup" icon={<Search size={15} />} value={opponentChampionId} onChange={(value) => onOpponentChange(Number(value))}>
        <option value={0}>vs. Champion...</option>
        {champions.map((champion) => (
          <option key={champion.key} value={champion.key}>{champion.name}</option>
        ))}
      </SelectControl>
    </div>
  );
}

function mergeGuideWithBuildAdvice(guide: ChampionGuideResponse | undefined, buildAdvice: BuildAdviceResponse): ChampionGuideResponse {
  return {
    summary: buildAdvice.champion.summary,
    toughestMatchups: guide?.toughestMatchups ?? [],
    bestMatchups: guide?.bestMatchups ?? [],
    topRunes: buildAdvice.champion.topRunes,
    topSpells: buildAdvice.champion.topSpells,
    topSkillOrders: guide?.topSkillOrders ?? [],
    topItemPaths: buildAdvice.champion.topItemPaths,
    buildVariants: buildAdvice.champion.buildVariants ?? guide?.buildVariants ?? [],
  };
}

function buildAdviceContext(buildAdvice: BuildAdviceResponse | undefined, opponentName?: string) {
  if (!buildAdvice) {
    return opponentName ? `Filtered into ${opponentName}` : 'Champion-wide build path';
  }
  if (buildAdvice.matchup.available && buildAdvice.matchup.sample.fallbackUsed) {
    return opponentName ? `Into ${opponentName}, widened where samples are thin` : 'Champion-wide build path';
  }
  if (buildAdvice.matchup.available) {
    return opponentName ? `Filtered into ${opponentName}` : 'Champion-wide build path';
  }
  return 'Champion-wide build path';
}

function GuideCoverage({ loading, championCount, totalGames, selectedGames, role, rankLabel, patch }: { loading: boolean; championCount: number; totalGames: number; selectedGames: number; role: string; rankLabel: string; patch: string }) {
  return (
    <div className="guide-coverage-strip">
      <div className="guide-coverage-title">
        <Database size={15} />
        <span>Stored Coverage</span>
      </div>
      <MetricTile className="guide-coverage-stat" label="Champions with data" value={loading ? '...' : formatNumber(championCount)} />
      <MetricTile className="guide-coverage-stat" label={`${role} games indexed`} value={loading ? '...' : formatNumber(totalGames)} />
      <MetricTile className="guide-coverage-stat" label="Selected champion" value={loading ? '...' : formatNumber(selectedGames)} />
      <MetricTile className="guide-coverage-stat" label="Scope" value={`${patch || 'all patches'} / ${rankLabel}`} />
    </div>
  );
}

function GuideStats({ guide, loading }: { guide?: ChampionGuideResponse; loading: boolean }) {
  const summary = guide?.summary;
  return (
    <div className="guide-stat-strip">
      <MetricTile className="guide-stat" label="Tier" value={loading ? '...' : guideTier(guide)} tone="tier" />
      <MetricTile className="guide-stat" label="Win Rate" value={loading ? '...' : summary?.games ? `${summary.winRate.toFixed(2)}%` : '--'} />
      <MetricTile className="guide-stat" label="Rank" value={loading ? '...' : summary?.roleRank ? `${summary.roleRank} / ${summary.roleRankTotal}` : '--'} />
      <MetricTile className="guide-stat" label="Pick Rate" value={loading ? '...' : summary?.games ? `${summary.pickRate.toFixed(2)}%` : '--'} />
      <MetricTile className="guide-stat" label="Ban Rate" value={loading ? '...' : summary?.banRate ? `${summary.banRate.toFixed(2)}%` : '--'} />
      <MetricTile className="guide-stat" label="Confidence" value={loading ? '...' : summary?.games ? `${summary.confidence.toFixed(1)}%` : '--'} />
      <MetricTile className="guide-stat" label="Matches" value={loading ? '...' : formatNumber(summary?.games ?? 0)} />
    </div>
  );
}

function RoleQuestCard() {
  return (
    <PanelCard className="guide-card guide-role-quest">
      <PanelTitle title="Role Quest" detail="Jungle context" />
      <p>Jungle builds include starter jungle items when they appear in the collected purchase path. This keeps jungle-specific first-slot data separate from lane builds.</p>
    </PanelCard>
  );
}

function GuideMiniNote({ title, body }: { title: string; body: string }) {
  return (
    <PanelCard className="guide-card guide-mini-note">
      <PanelTitle title={title} />
      <p>{body}</p>
    </PanelCard>
  );
}

function AbilityIcon({ champions, ability, label, folder = 'passive' }: { champions?: ChampionData; ability?: { name?: string; image?: { full: string } }; label: string; folder?: 'passive' | 'spell' }) {
  const src = championAbilityImageUrl(champions, ability?.image?.full, folder);
  return (
    <span className="guide-ability-icon">
      {src ? <img src={src} alt={ability?.name ?? label} title={ability?.name ?? label} /> : <Shield size={19} />}
      <b>{label}</b>
    </span>
  );
}

function guideTier(guide?: ChampionGuideResponse) {
  return championTier(guide?.summary);
}

function guideTierClassName(tier: string) {
  if (tier === 'S+') return 'guide-tier-s-plus';
  return `guide-tier-${tier.toLowerCase().replace(/[^a-z0-9]+/g, '-') || 'unknown'}`;
}

function formatNumber(value: number) {
  return new Intl.NumberFormat('en-US').format(value);
}

function variantItemSlots(variant: ChampionGuideBuildVariant): GuideItemSlot[] {
  const path = signatureItems(variant.core3Signature);
  for (const itemId of signatureItems(variant.finalItemsSignature)) {
    if (!path.includes(itemId)) {
      path.push(itemId);
    }
  }
  return path.slice(0, 6).map((itemId, index) => ({
    itemSlot: index + 1,
    itemId,
    wins: variant.wins,
    games: variant.games,
    winRate: variant.winRate,
    confidence: variant.confidence,
  }));
}

function withStartingItemRows(startingSource: GuideItemSlot[], buildRows: GuideItemSlot[]) {
  const startingRows = startingSource.filter((row) => row.itemSlot === 0);
  return startingRows.length ? [...startingRows, ...buildRows] : buildRows;
}

function signatureItems(signature: string) {
  return signature
    .split('-')
    .map((part) => Number(part))
    .filter((itemId) => itemId > 0);
}
