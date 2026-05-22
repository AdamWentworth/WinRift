import { Database, Filter, Search, Shield } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import type { CSSProperties } from 'react';
import { useQuery } from '@tanstack/react-query';
import { getBuildAdvice, getChampionGuide, getChampionGuideIndex } from '../api/client';
import type { AnalyticsItemSlot, BuildAdviceResponse, Champion, ChampionData, ChampionGuideBuildVariant, ChampionGuideResponse, ChampionGuideSummary, ItemData, RuneData, RuneStyle, SummonerSpellData } from '../api/types';
import {
  championAbilityImageUrl,
  championByKey,
  championImageUrl,
  championList,
  championSplashUrl,
  itemImageUrl,
  itemName,
  parseRuneSignature,
  runeImageUrl,
  runeStyleImageUrl,
  signatureSpells,
  summonerSpellImageUrl,
  summonerSpellName,
} from '../lib/staticData';
import { ROLE_OPTIONS, RoleIcon, roleLabel } from '../lib/roles';
import { championTier } from '../lib/tiers';
import { MetricTile } from './ui/MetricTile';
import { EmptyState, PanelCard, PanelTitle } from './ui/Panel';
import { RoleTabs } from './ui/RoleTabs';
import { SelectControl } from './ui/SelectControl';
import { StatShardGrid } from './ui/StatShardDisplay';

const ranks = [
  { value: '', label: 'All Ranks' },
  { value: 'MASTER+', label: 'Master+' },
  { value: 'DIAMOND', label: 'Diamond' },
  { value: 'EMERALD', label: 'Emerald' },
  { value: 'PLATINUM', label: 'Platinum' },
  { value: 'GOLD', label: 'Gold' },
];

const RECOMMENDED_BUILD_KEY = 'recommended';

type Props = {
  champions?: ChampionData;
  items?: ItemData;
  spells?: SummonerSpellData;
  runes?: RuneData;
  initialChampionId?: number;
  onChampionChange?: (champion: Champion) => void;
};

type GuideItemSlot = Pick<AnalyticsItemSlot, 'itemSlot' | 'itemId' | 'wins' | 'games' | 'winRate' | 'confidence'>;

export function BuildGuidePage({ champions, items, spells, runes, initialChampionId, onChampionChange }: Props) {
  const championsByName = useMemo(() => championList(champions), [champions]);
  const defaultChampionId = useMemo(() => {
    const wukong = championsByName.find((champion) => champion.id === 'MonkeyKing');
    return Number(wukong?.key ?? championsByName[0]?.key ?? 62);
  }, [championsByName]);
  const [championId, setChampionId] = useState(initialChampionId ?? defaultChampionId);
  const [role, setRole] = useState('JUNGLE');
  const [rankBucket, setRankBucket] = useState('');
  const [opponentChampionId, setOpponentChampionId] = useState(0);
  const [selectedBuildVariantKey, setSelectedBuildVariantKey] = useState('');
  const patch = patchBucket(champions?.version);
  const champion = championByKey(champions, championId);
  const opponent = opponentChampionId ? championByKey(champions, opponentChampionId) : undefined;
  const itemContext = itemContextForRole(role);
  const guideQuery = useQuery({
    queryKey: ['champion-guide', championId, role, patch, rankBucket],
    queryFn: () => getChampionGuide({ championId, role, patch, rankBucket, minGames: 5, limit: 12 }),
    enabled: Boolean(championId && patch),
    staleTime: 5 * 60 * 1000,
  });
  const buildAdviceQuery = useQuery({
    queryKey: ['guide-build-advice', championId, role, patch, rankBucket, opponentChampionId],
    queryFn: () => getBuildAdvice({
      championId,
      role,
      itemContext,
      opponentChampionId: opponentChampionId || undefined,
      patch,
      rankBucket,
      minGames: opponentChampionId ? 3 : 5,
      championMinGames: 10,
      limit: 4,
    }),
    enabled: Boolean(championId && patch),
    staleTime: 5 * 60 * 1000,
  });

  useEffect(() => {
    if (initialChampionId && championByKey(champions, initialChampionId) && initialChampionId !== championId) {
      setChampionId(initialChampionId);
      return;
    }
    if (defaultChampionId && !championByKey(champions, championId)) {
      setChampionId(defaultChampionId);
    }
  }, [champions, championId, defaultChampionId, initialChampionId]);

  useEffect(() => {
    setSelectedBuildVariantKey('');
  }, [championId, role, rankBucket]);

  const updateChampion = (value: number) => {
    setChampionId(value);
    const nextChampion = championByKey(champions, value);
    if (nextChampion) {
      onChampionChange?.(nextChampion);
    }
  };

  const guide = guideQuery.data;
  const buildAdvice = buildAdviceQuery.data;
  const guideForBuilds = buildAdvice ? mergeGuideWithBuildAdvice(guide, buildAdvice) : guide;
  const buildVariants = useMemo(() => groupBuildVariantsForDisplay(guideForBuilds?.buildVariants ?? [], items), [guideForBuilds?.buildVariants, items]);
  const selectedBuildVariant = selectedBuildVariantKey && selectedBuildVariantKey !== RECOMMENDED_BUILD_KEY
    ? buildVariants.find((variant) => variant.variantKey === selectedBuildVariantKey)
    : undefined;
  const selectedBuildVariantIndex = selectedBuildVariant ? buildVariants.indexOf(selectedBuildVariant) : -1;
  const selectedBuildVariantLabel = selectedBuildVariant ? buildVariantLabel(selectedBuildVariant, selectedBuildVariantIndex, items) : '';
  const selectedVariantItemSlots = selectedBuildVariant ? variantItemSlots(selectedBuildVariant) : [];
  const itemSlots: GuideItemSlot[] = selectedVariantItemSlots.length
    ? selectedVariantItemSlots
    : buildAdvice?.matchup.available && buildAdvice.matchup.itemSlots.length
      ? buildAdvice.matchup.itemSlots
      : buildAdvice?.champion.itemSlots ?? [];
  const itemSlotContext = selectedVariantItemSlots.length
    ? `${selectedBuildVariantLabel} selected build family`
    : buildAdviceContext(buildAdvice, opponent?.name);
  const splash = championSplashUrl(champions, championId);
  const rankLabel = ranks.find((candidate) => candidate.value === rankBucket)?.label ?? 'All Ranks';
  const titleRole = roleLabel(role);
  const sampleContext = opponent ? `${champion?.name ?? championId} vs ${opponent.name}` : `${champion?.name ?? championId} overall`;
  const guideIndexQuery = useQuery({
    queryKey: ['champion-guide-index', role, patch, rankBucket],
    queryFn: () => getChampionGuideIndex({ role, patch, rankBucket, minGames: 1, limit: 250 }),
    enabled: Boolean(patch),
    staleTime: 5 * 60 * 1000,
  });
  const guideIndex = guideIndexQuery.data?.results ?? [];
  const coverageByChampionId = useMemo(() => {
    const map = new Map<number, ChampionGuideSummary>();
    for (const summary of guideIndex) {
      map.set(summary.championId, summary);
    }
    return map;
  }, [guideIndex]);
  const currentCoverage = coverageByChampionId.get(championId);
  const coveredGames = guideIndex.reduce((total, summary) => total + summary.games, 0);

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
              <span>{champion?.name ?? 'Champion'}</span> <em><RoleIcon role={role} /> {titleRole} patterns</em>
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
          <span>Patch {patch || champions?.version || 'current'}</span>
          <b>{rankLabel}</b>
          <em>{formatNumber(currentCoverage?.games ?? 0)} role games</em>
        </div>
      </div>

      <GuideFilters
        champions={championsByName}
        coverage={coverageByChampionId}
        championId={championId}
        role={role}
        rankBucket={rankBucket}
        opponentChampionId={opponentChampionId}
        onChampionChange={updateChampion}
        onRoleChange={setRole}
        onRankChange={setRankBucket}
        onOpponentChange={setOpponentChampionId}
      />

      <GuideStats guide={guideForBuilds} loading={guideQuery.isLoading && buildAdviceQuery.isLoading} />
      <GuideCoverage
        loading={guideIndexQuery.isLoading}
        championCount={guideIndex.length}
        totalGames={coveredGames}
        selectedGames={currentCoverage?.games ?? 0}
        role={titleRole}
        rankLabel={rankLabel}
        patch={patch}
      />

      <div className="guide-context-banner">
        <span>Current Sample</span>
        <b>{sampleContext}</b>
        <em>{opponent ? 'matchup-filtered where possible, then widened carefully' : 'champion-wide until a matchup is selected'}</em>
      </div>

      <BuildVariantTabs
        variants={buildVariants}
        recommendedGuide={guideForBuilds}
        selectedKey={selectedBuildVariant?.variantKey ?? RECOMMENDED_BUILD_KEY}
        items={items}
        onSelect={setSelectedBuildVariantKey}
      />

      <div className="guide-primary-grid">
        <RuneGuideCard guide={guideForBuilds} variant={selectedBuildVariant} runes={runes} loading={buildAdviceQuery.isLoading || guideQuery.isLoading} />
        <div className="guide-side-stack">
          <SpellGuideCard guide={guideForBuilds} variant={selectedBuildVariant} spells={spells} loading={buildAdviceQuery.isLoading || guideQuery.isLoading} />
          <GuideMiniNote
            title="Matchup Lens"
            body={buildAdvice?.notes[0] ?? (opponent ? `Item panels are narrowed to ${opponent.name} when the sample exists, then fall back to broader ${champion?.name ?? 'champion'} data.` : 'Choose a matchup filter to compare item choices into a specific opponent.')}
          />
        </div>
      </div>

      <MatchupStrip title="Toughest Matchups" subtitle={`These champions have performed best into ${champion?.name ?? 'this champion'}`} rows={guide?.toughestMatchups ?? []} champions={champions} tone="bad" loading={guideQuery.isLoading} />

      <div className="guide-skill-items-row">
        <SkillGuideCard guide={guide} champion={champion} champions={champions} loading={guideQuery.isLoading} />
        <SkillPathCard guide={guide} championName={champion?.name ?? 'this champion'} loading={guideQuery.isLoading} />
      </div>

      <ItemPathSummaryCard guide={guideForBuilds} variant={selectedBuildVariant} items={items} loading={buildAdviceQuery.isLoading || guideQuery.isLoading} />

      <BuildAdviceCoverage buildAdvice={buildAdvice} loading={buildAdviceQuery.isLoading} />

      <ItemGuideGrid rows={itemSlots} items={items} loading={buildAdviceQuery.isLoading} context={itemSlotContext} />

      {role === 'JUNGLE' ? <RoleQuestCard /> : null}

      <MatchupStrip title="Favorable Matchups" subtitle={`${champion?.name ?? 'This champion'} has performed well into these opponents`} rows={guide?.bestMatchups ?? []} champions={champions} tone="good" loading={guideQuery.isLoading} />
    </section>
  );
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

function BuildVariantTabs({
  variants,
  recommendedGuide,
  selectedKey,
  items,
  onSelect,
}: {
  variants: ChampionGuideBuildVariant[];
  recommendedGuide?: ChampionGuideResponse;
  selectedKey: string;
  items?: ItemData;
  onSelect: (key: string) => void;
}) {
  if (!variants.length && !recommendedGuide) return null;
  const recommendedPath = recommendedGuide?.topItemPaths[0];
  const recommendedSummary = recommendedGuide?.summary;
  return (
    <div className="guide-build-variant-tabs" role="tablist" aria-label="Build variants">
      <button
        key={RECOMMENDED_BUILD_KEY}
        className={selectedKey === RECOMMENDED_BUILD_KEY ? 'active' : ''}
        type="button"
        role="tab"
        aria-selected={selectedKey === RECOMMENDED_BUILD_KEY}
        onClick={() => onSelect('')}
      >
        <span>Recommended</span>
        <ItemSignatureImages signature={recommendedPath?.core3Signature ?? recommendedPath?.finalItemsSignature ?? ''} items={items} limit={2} />
        <em>{recommendedSummary?.games ? `${recommendedSummary.winRate.toFixed(1)}% · ${formatNumber(recommendedSummary.games)}` : 'All collected data'}</em>
      </button>
      {variants.slice(0, 5).map((variant, index) => (
        <button
          key={variant.variantKey}
          className={variant.variantKey === selectedKey ? 'active' : ''}
          type="button"
          role="tab"
          aria-selected={variant.variantKey === selectedKey}
          onClick={() => onSelect(variant.variantKey)}
        >
          <span>{buildVariantLabel(variant, index, items)}</span>
          <ItemSignatureImages signature={variant.core2Signature || variant.core3Signature} items={items} limit={2} />
          <em>{variant.winRate.toFixed(1)}% · {formatNumber(variant.games)}</em>
        </button>
      ))}
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
      <MetricTile className="guide-stat" label="Win Rate" value={summary?.games ? `${summary.winRate.toFixed(2)}%` : '--'} />
      <MetricTile className="guide-stat" label="Rank" value={summary?.roleRank ? `${summary.roleRank} / ${summary.roleRankTotal}` : '--'} />
      <MetricTile className="guide-stat" label="Pick Rate" value={summary?.games ? `${summary.pickRate.toFixed(2)}%` : '--'} />
      <MetricTile className="guide-stat" label="Ban Rate" value={summary?.banRate ? `${summary.banRate.toFixed(2)}%` : '--'} />
      <MetricTile className="guide-stat" label="Confidence" value={summary?.games ? `${summary.confidence.toFixed(1)}%` : '--'} />
      <MetricTile className="guide-stat" label="Matches" value={formatNumber(summary?.games ?? 0)} />
    </div>
  );
}

function RuneGuideCard({ guide, variant, runes, loading }: { guide?: ChampionGuideResponse; variant?: ChampionGuideBuildVariant; runes?: RuneData; loading: boolean }) {
  const runeRow = variant?.runeSignature ? variantRuneRow(variant) : guide?.topRunes[0];
  const parsed = parseRuneSignature(runeRow?.runeSignature ?? '');
  const primary = runes?.data.find((style) => style.id === parsed.primaryStyleId);
  const secondary = runes?.data.find((style) => style.id === parsed.secondaryStyleId);
  return (
    <PanelCard className="guide-card rune-guide-card">
      <PanelTitle title="Runes" detail={runeRow ? `${runeRow.winRate.toFixed(2)}% WR (${formatNumber(runeRow.games)} matches)` : loading ? 'Loading...' : 'No rune sample yet'} />
      {runeRow ? (
        <div className="guide-rune-grid">
          <RuneTreePanel style={primary} selectedRuneIds={parsed.runeIds} runes={runes} treeRole="primary" />
          <div className="guide-rune-side">
            <RuneTreePanel style={secondary} selectedRuneIds={parsed.runeIds} runes={runes} treeRole="secondary" />
            <StatShardGrid selectedIds={parsed.statPerks} className="guide-stat-shards" />
          </div>
        </div>
      ) : <EmptyState message="Rune pages will appear once this champion has enough collected games." />}
    </PanelCard>
  );
}

function RuneTreePanel({ style, selectedRuneIds, runes, treeRole }: { style?: RuneStyle; selectedRuneIds: number[]; runes?: RuneData; treeRole: 'primary' | 'secondary' }) {
  const selected = new Set(selectedRuneIds);
  const styleImage = style ? runeStyleImageUrl(runes, style.id) : '';
  const slots = treeRole === 'secondary' ? (style?.slots ?? []).slice(1) : (style?.slots ?? []);
  return (
    <div className={treeRole === 'secondary' ? 'guide-rune-tree secondary' : 'guide-rune-tree primary'}>
      <div className="guide-rune-tree-title">
        {styleImage ? <img src={styleImage} alt="" /> : null}
        <strong>{style?.name ?? 'Rune Tree'}</strong>
      </div>
      <div className="guide-rune-slots">
        {slots.map((slot, index) => {
          const originalIndex = treeRole === 'secondary' ? index + 1 : index;
          return (
            <div key={`${style?.id}-${originalIndex}`} className={originalIndex === 0 ? 'guide-rune-slot keystone' : 'guide-rune-slot'}>
              {slot.runes.map((rune) => {
                const active = selected.has(rune.id);
                const src = runeImageUrl(runes, rune.id);
                return src ? (
                  <img
                    key={rune.id}
                    className={active ? 'selected' : ''}
                    src={src}
                    alt={rune.name}
                    title={active ? `${rune.name} selected` : rune.name}
                  />
                ) : null;
              })}
            </div>
          );
        })}
      </div>
    </div>
  );
}

function SpellGuideCard({ guide, variant, spells, loading }: { guide?: ChampionGuideResponse; variant?: ChampionGuideBuildVariant; spells?: SummonerSpellData; loading: boolean }) {
  const spellRow = variant?.spellSignature ? variantSpellRow(variant) : guide?.topSpells[0];
  const spellIds = signatureSpells(spellRow?.spellSignature ?? '');
  return (
    <PanelCard className="guide-card spell-guide-card">
      <PanelTitle title="Summoner Spells" detail={spellRow ? `${spellRow.winRate.toFixed(2)}% WR (${formatNumber(spellRow.games)} matches)` : loading ? 'Loading...' : 'No spell sample yet'} />
      {spellIds.length ? (
        <div className="guide-spell-pair">
          {spellIds.map((spellId) => {
            const src = summonerSpellImageUrl(spells, spellId);
            return src ? <img key={spellId} src={src} alt={summonerSpellName(spells, spellId)} title={summonerSpellName(spells, spellId)} /> : null;
          })}
        </div>
      ) : <EmptyState message="Summoner spell samples will appear after collection." />}
    </PanelCard>
  );
}

function MatchupStrip({ title, subtitle, rows, champions, tone, loading }: { title: string; subtitle: string; rows: { opponentChampionId: number; winRate: number; games: number }[]; champions?: ChampionData; tone: 'good' | 'bad'; loading: boolean }) {
  return (
    <PanelCard as="section" className="guide-card matchup-strip">
      <PanelTitle title={title} detail={subtitle} />
      <div className="guide-matchup-row">
        {rows.length ? rows.map((row) => {
          const opponent = championByKey(champions, row.opponentChampionId);
          return (
            <div key={`${title}-${row.opponentChampionId}`} className={`guide-matchup-tile ${tone}`}>
              {opponent ? <img src={championImageUrl(champions, row.opponentChampionId)} alt={opponent.name} /> : null}
              <strong>{opponent?.name ?? row.opponentChampionId}</strong>
              <b>{row.winRate.toFixed(1)}%</b>
              <span>{formatNumber(row.games)} matches</span>
            </div>
          );
        }) : <EmptyState message={loading ? 'Loading matchups...' : 'No matchup sample yet for this filter.'} />}
      </div>
    </PanelCard>
  );
}

function SkillGuideCard({ guide, champion, champions, loading }: { guide?: ChampionGuideResponse; champion?: Champion; champions?: ChampionData; loading: boolean }) {
  const spells = champion?.spells ?? [];
  const skillOrder = guide?.topSkillOrders?.[0];
  const priority = skillPriority(skillOrder?.skillOrderSignature ?? '');
  return (
    <PanelCard className="guide-card skill-priority-card">
      <PanelTitle title="Skill Priority" detail={skillOrder ? `${skillOrder.winRate.toFixed(2)}% WR (${formatNumber(skillOrder.games)} matches)` : loading ? 'Loading...' : 'No skill sample yet'} />
      <div className="skill-priority-icons">
        {(priority.length ? priority : [1, 2, 3]).map((slot) => {
          const ability = spells[slot - 1];
          return (
            <span key={slot} className={priority.length ? '' : 'muted-skill'}>
              {ability ? <img src={championAbilityImageUrl(champions, ability.image?.full, 'spell')} alt={ability.name} title={ability.name} /> : null}
              <b>{skillLabel(slot)}</b>
            </span>
          );
        })}
      </div>
      {skillOrder ? (
        <p>Derived from Match-V5 timeline skill-level events in collected ranked games.</p>
      ) : (
        <p>{loading ? 'Checking collected timelines.' : 'Skill order will appear after guide analytics refreshes for this champion.'}</p>
      )}
    </PanelCard>
  );
}

function SkillPathCard({ guide, championName, loading }: { guide?: ChampionGuideResponse; championName: string; loading: boolean }) {
  const skillOrder = guide?.topSkillOrders?.[0];
  const slots = skillSlots(skillOrder?.skillOrderSignature ?? '');
  return (
    <PanelCard className="guide-card skill-path-card">
      <PanelTitle title="Skill Path" detail={skillOrder ? `${formatNumber(skillOrder.games)} matching paths` : loading ? 'Loading...' : 'No path sample yet'} />
      {slots.length ? (
        <div className="skill-path-placeholder-grid">
          {[1, 2, 3, 4].map((slot) => (
            <div key={slot}>
              <b>{skillLabel(slot)}</b>
              {Array.from({ length: 18 }, (_, index) => {
                const level = index + 1;
                const active = slots[index] === slot;
                return <span key={level} className={active ? 'lit' : ''}>{active ? level : ''}</span>;
              })}
            </div>
          ))}
        </div>
      ) : <EmptyState message={loading ? 'Loading skill path...' : `No skill path sample yet for ${championName}.`} />}
      <p>{slots.length ? `${championName} skill order is shown from the most common collected path.` : 'This panel stays empty until real timeline data exists.'}</p>
    </PanelCard>
  );
}

function ItemGuideGrid({ rows, items, loading, context }: { rows: GuideItemSlot[]; items?: ItemData; loading: boolean; context: string }) {
  const slotRows = [1, 2, 3, 4, 5, 6].map((slot) => rows.filter((row) => row.itemSlot === slot));
  const coreRows = slotRows.slice(0, 3).map((candidates) => candidates[0]).filter(Boolean) as GuideItemSlot[];
  return (
    <section className="guide-item-grid">
      <GuideItemPanel title="First Slot" subtitle={context} rows={slotRows[0]?.slice(0, 1) ?? []} items={items} loading={loading} />
      <GuideItemPanel title="Core Items" subtitle="Highest-confidence path" rows={coreRows} items={items} loading={loading} linked />
      <GuideItemPanel title="Fourth Item Options" subtitle="Options after core" rows={slotRows[3]?.slice(0, 3) ?? []} items={items} loading={loading} />
      <GuideItemPanel title="Fifth Item Options" subtitle="Late build pivots" rows={slotRows[4]?.slice(0, 3) ?? []} items={items} loading={loading} />
      <GuideItemPanel title="Sixth Item Options" subtitle="Full-build finishers" rows={slotRows[5]?.slice(0, 3) ?? []} items={items} loading={loading} />
    </section>
  );
}

function ItemPathSummaryCard({ guide, variant, items, loading }: { guide?: ChampionGuideResponse; variant?: ChampionGuideBuildVariant; items?: ItemData; loading: boolean }) {
  const selectedPath = variant ? variantItemPathRow(variant) : undefined;
  const rows = selectedPath ? [selectedPath, ...(guide?.topItemPaths ?? []).filter((row) => row.core3Signature !== selectedPath.core3Signature).slice(0, 2)] : guide?.topItemPaths ?? [];
  return (
    <PanelCard className="guide-card item-path-summary-card">
      <PanelTitle
        title="Item Paths"
        detail={rows[0] ? `${rows[0].winRate.toFixed(2)}% WR (${formatNumber(rows[0].games)} matches)${variant ? ' for selected build family' : ' for the strongest complete path'}` : loading ? 'Loading...' : 'No complete path sample yet'}
      />
      {rows.length ? (
        <div className="guide-build-path-list">
          {rows.slice(0, 3).map((row, index) => (
            <div key={`${row.core3Signature}-${row.finalItemsSignature}`} className="guide-build-path-card">
              <div className="guide-build-path-rank">{index + 1}</div>
              <div className="guide-build-path-main">
                <div className="guide-build-path-line">
                  <span>Core</span>
                  <ItemSignatureImages signature={row.core3Signature} items={items} />
                </div>
                <div className="guide-build-path-line final">
                  <span>Final</span>
                  <ItemSignatureImages signature={row.finalItemsSignature} items={items} limit={6} />
                </div>
              </div>
              <div className="guide-build-path-stats">
                <b>{row.winRate.toFixed(2)}%</b>
                <span>{formatNumber(row.games)} games</span>
                <em>{row.confidence.toFixed(1)} confidence</em>
              </div>
            </div>
          ))}
        </div>
      ) : (
        <EmptyState message={loading ? 'Loading item paths...' : 'Complete item paths will appear after this champion has enough collected games.'} />
      )}
    </PanelCard>
  );
}

function ItemSignatureImages({ signature, items, limit }: { signature: string; items?: ItemData; limit?: number }) {
  const itemIds = signatureItems(signature).slice(0, limit ?? 99);
  return (
    <div className="guide-build-path-icons">
      {itemIds.length ? itemIds.map((itemId) => {
        const src = itemImageUrl(items, String(itemId));
        return src ? (
          <img key={`${signature}-${itemId}`} src={src} alt={itemName(items, String(itemId))} title={itemName(items, String(itemId))} />
        ) : (
          <span key={`${signature}-${itemId}`} className="item-pill">{itemId}</span>
        );
      }) : <em>No items</em>}
    </div>
  );
}

function GuideItemPanel({ title, subtitle, rows, items, loading, linked }: { title: string; subtitle: string; rows: GuideItemSlot[]; items?: ItemData; loading: boolean; linked?: boolean }) {
  return (
    <PanelCard className="guide-card guide-item-panel">
      <PanelTitle title={title} />
      {rows.length ? (
        <div className={linked ? 'guide-item-list linked' : 'guide-item-list'}>
          {rows.map((row, index) => (
            <div key={`${title}-${row.itemSlot}-${row.itemId}`} className="guide-item-option">
              {index > 0 && linked ? <span className="guide-item-arrow">-&gt;</span> : null}
              {itemImageUrl(items, String(row.itemId)) ? (
                <img src={itemImageUrl(items, String(row.itemId))} alt={itemName(items, String(row.itemId))} title={itemName(items, String(row.itemId))} />
              ) : (
                <span className="item-pill">{row.itemId}</span>
              )}
              <div>
                <strong>{row.winRate.toFixed(2)}% WR</strong>
                <span>{formatNumber(row.games)} matches</span>
              </div>
            </div>
          ))}
        </div>
      ) : <EmptyState message={loading ? 'Loading items...' : 'No item sample yet.'} />}
      <small>{subtitle}</small>
    </PanelCard>
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

function itemContextForRole(role: string): 'JUNGLE' | 'SUPPORT' | undefined {
  if (role === 'JUNGLE') return 'JUNGLE';
  if (role === 'UTILITY') return 'SUPPORT';
  return undefined;
}

function patchBucket(version?: string) {
  const parts = (version ?? '').split('.');
  if (parts.length >= 2) {
    return `${parts[0]}.${parts[1]}`;
  }
  return '';
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

function groupBuildVariantsForDisplay(variants: ChampionGuideBuildVariant[], items?: ItemData) {
  const groups = new Map<string, ChampionGuideBuildVariant>();
  const representativeGames = new Map<string, number>();
  for (const variant of variants) {
    const familyLabel = buildVariantFamilyLabel(variant, items);
    const key = familyLabel ? `label:${familyLabel.toLowerCase().replace(/[^a-z0-9]+/g, '-')}` : `core:${variant.variantKey}`;
    const existing = groups.get(key);
    if (!existing) {
      groups.set(key, { ...variant, variantKey: key, variantLabel: familyLabel || variant.variantLabel });
      representativeGames.set(key, variant.games);
      continue;
    }
    const wins = existing.wins + variant.wins;
    const games = existing.games + variant.games;
    const next: ChampionGuideBuildVariant = {
      ...existing,
      wins,
      games,
      winRate: games ? (wins / games) * 100 : 0,
      confidence: Math.max(existing.confidence, variant.confidence),
      buildCount: existing.buildCount + variant.buildCount,
      variantTags: Array.from(new Set([...(existing.variantTags ?? []), ...(variant.variantTags ?? [])])),
    };
    if (variant.games > (representativeGames.get(key) ?? 0)) {
      next.core2Signature = variant.core2Signature;
      next.core3Signature = variant.core3Signature;
      next.finalItemsSignature = variant.finalItemsSignature;
      next.runeSignature = variant.runeSignature;
      next.spellSignature = variant.spellSignature;
      representativeGames.set(key, variant.games);
    }
    groups.set(key, next);
  }
  return Array.from(groups.values()).sort((a, b) => {
    if (a.games !== b.games) return b.games - a.games;
    return b.winRate - a.winRate;
  });
}

function variantRuneRow(variant: ChampionGuideBuildVariant) {
  return {
    runeSignature: variant.runeSignature,
    wins: variant.wins,
    games: variant.games,
    winRate: variant.winRate,
    confidence: variant.confidence,
  };
}

function variantSpellRow(variant: ChampionGuideBuildVariant) {
  return {
    spellSignature: variant.spellSignature,
    wins: variant.wins,
    games: variant.games,
    winRate: variant.winRate,
    confidence: variant.confidence,
  };
}

function variantItemPathRow(variant: ChampionGuideBuildVariant) {
  return {
    core3Signature: variant.core3Signature,
    finalItemsSignature: variant.finalItemsSignature,
    wins: variant.wins,
    games: variant.games,
    winRate: variant.winRate,
    confidence: variant.confidence,
  };
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

function buildVariantLabel(variant: ChampionGuideBuildVariant, index: number, items?: ItemData) {
  const familyLabel = buildVariantFamilyLabel(variant, items);
  if (familyLabel) return familyLabel;
  return `Variant ${index + 1}`;
}

function buildVariantFamilyLabel(variant: ChampionGuideBuildVariant, items?: ItemData) {
  if (variant.variantLabel) return variant.variantLabel;
  const names = signatureItems(`${variant.core2Signature}-${variant.core3Signature}-${variant.finalItemsSignature}`)
    .map((itemId) => itemName(items, String(itemId)).toLowerCase());
  const has = (...needles: string[]) => names.some((name) => needles.some((needle) => name.includes(needle)));
  if (has('heartsteel', "jak'sho", 'randuin', 'spirit visage', 'thornmail', 'sunfire', 'unending', 'dead man', 'kaenic', 'rookern')) {
    return 'Tank';
  }
  if (has("guinsoo", "nashor", 'ruined king', "wit'", 'terminus')) {
    return 'On Hit';
  }
  if (has('kraken', 'collector', 'infinity edge', 'serylda', "youmuu", 'eclipse', 'opportunity', 'axiom', 'edge of night', 'lord dominik')) {
    return 'AD';
  }
  if (has('rabadon', 'shadowflame', 'stormsurge', 'lich bane', 'void staff', 'zhonya', 'liandry', 'luden', 'malignance')) {
    return 'AP';
  }
  const coreNames = signatureItems(variant.core2Signature)
    .map((itemId) => itemName(items, String(itemId)))
    .filter((name) => !/^\d+$/.test(name));
  return coreNames[0] ? coreNames[0].replace(/'s\b.*$/, '') : '';
}

function skillSlots(signature: string) {
  return signature
    .split('-')
    .map((part) => Number(part))
    .filter((slot) => slot >= 1 && slot <= 4);
}

function signatureItems(signature: string) {
  return signature
    .split('-')
    .map((part) => Number(part))
    .filter((itemId) => itemId > 0);
}

function skillPriority(signature: string) {
  const slots = skillSlots(signature).filter((slot) => slot >= 1 && slot <= 3);
  const counts = [1, 2, 3].map((slot) => ({
    slot,
    count: slots.filter((candidate) => candidate === slot).length,
    firstIndex: slots.indexOf(slot) === -1 ? 99 : slots.indexOf(slot),
  }));
  return counts
    .filter((row) => row.count > 0)
    .sort((a, b) => {
      if (a.count !== b.count) return b.count - a.count;
      return a.firstIndex - b.firstIndex;
    })
    .map((row) => row.slot);
}

function skillLabel(slot: number) {
  return ['?', 'Q', 'W', 'E', 'R'][slot] ?? '?';
}
