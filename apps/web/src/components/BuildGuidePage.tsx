import { Database, Filter, Search, Shield } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import type { CSSProperties } from 'react';
import { useQuery } from '@tanstack/react-query';
import { getChampionGuide, getChampionGuideIndex, getItemSlots } from '../api/client';
import type { AnalyticsItemSlot, Champion, ChampionData, ChampionGuideResponse, ChampionGuideSummary, ItemData, RuneData, RuneStyle, SummonerSpellData } from '../api/types';
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
import { MetricTile } from './ui/MetricTile';
import { EmptyState, PanelCard, PanelTitle } from './ui/Panel';
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

type Props = {
  champions?: ChampionData;
  items?: ItemData;
  spells?: SummonerSpellData;
  runes?: RuneData;
  initialChampionId?: number;
  onChampionChange?: (champion: Champion) => void;
};

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
  const itemSlotsQuery = useQuery({
    queryKey: ['guide-item-slots', championId, role, patch, rankBucket, opponentChampionId],
    queryFn: () => getItemSlots({
      championId,
      role,
      itemContext,
      opponentChampionId: opponentChampionId || undefined,
      patch,
      rankBucket,
      minGames: opponentChampionId ? 3 : 5,
      limit: 4,
      fallback: true,
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

  const updateChampion = (value: number) => {
    setChampionId(value);
    const nextChampion = championByKey(champions, value);
    if (nextChampion) {
      onChampionChange?.(nextChampion);
    }
  };

  const guide = guideQuery.data;
  const itemSlots = itemSlotsQuery.data?.results ?? [];
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
            <span className={`guide-tier ${guideTier(guide)}`}>{guideTier(guide)}</span>
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

      <GuideStats guide={guide} loading={guideQuery.isLoading} />
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

      <div className="guide-primary-grid">
        <RuneGuideCard guide={guide} runes={runes} loading={guideQuery.isLoading} />
        <div className="guide-side-stack">
          <SpellGuideCard guide={guide} spells={spells} loading={guideQuery.isLoading} />
          <GuideMiniNote
            title="Matchup Lens"
            body={opponent ? `Item panels are narrowed to ${opponent.name} when the sample exists, then fall back to broader ${champion?.name ?? 'champion'} data.` : 'Choose a matchup filter to compare item choices into a specific opponent.'}
          />
        </div>
      </div>

      <MatchupStrip title="Toughest Matchups" subtitle={`These champions have performed best into ${champion?.name ?? 'this champion'}`} rows={guide?.toughestMatchups ?? []} champions={champions} tone="bad" loading={guideQuery.isLoading} />

      <div className="guide-skill-items-row">
        <SkillGuideCard guide={guide} champion={champion} champions={champions} loading={guideQuery.isLoading} />
        <SkillPathCard guide={guide} championName={champion?.name ?? 'this champion'} loading={guideQuery.isLoading} />
      </div>

      <ItemPathSummaryCard guide={guide} items={items} loading={guideQuery.isLoading} />

      <ItemGuideGrid rows={itemSlots} items={items} loading={itemSlotsQuery.isLoading} context={opponent ? `Filtered into ${opponent.name}` : 'Champion-wide build path'} />

      {role === 'JUNGLE' ? <RoleQuestCard /> : null}

      <MatchupStrip title="Favorable Matchups" subtitle={`${champion?.name ?? 'This champion'} has performed well into these opponents`} rows={guide?.bestMatchups ?? []} champions={champions} tone="good" loading={guideQuery.isLoading} />
    </section>
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

function RuneGuideCard({ guide, runes, loading }: { guide?: ChampionGuideResponse; runes?: RuneData; loading: boolean }) {
  const runeRow = guide?.topRunes[0];
  const parsed = parseRuneSignature(runeRow?.runeSignature ?? '');
  const primary = runes?.data.find((style) => style.id === parsed.primaryStyleId);
  const secondary = runes?.data.find((style) => style.id === parsed.secondaryStyleId);
  return (
    <PanelCard className="guide-card rune-guide-card">
      <PanelTitle title="Runes" detail={runeRow ? `${runeRow.winRate.toFixed(2)}% WR (${formatNumber(runeRow.games)} matches)` : loading ? 'Loading...' : 'No rune sample yet'} />
      {runeRow ? (
        <div className="guide-rune-grid">
          <RuneTreePanel style={primary} selectedRuneIds={parsed.runeIds} runes={runes} />
          <RuneTreePanel style={secondary} selectedRuneIds={parsed.runeIds} runes={runes} />
          <div className="guide-stat-shards">
            <span>Stat Shards</span>
            <div>
              {parsed.statPerks.length ? parsed.statPerks.map((perk) => <b key={perk}>{statPerkLabel(perk)}</b>) : <b>Not available</b>}
            </div>
          </div>
        </div>
      ) : <EmptyState message="Rune pages will appear once this champion has enough collected games." />}
    </PanelCard>
  );
}

function RuneTreePanel({ style, selectedRuneIds, runes }: { style?: RuneStyle; selectedRuneIds: number[]; runes?: RuneData }) {
  const selected = new Set(selectedRuneIds);
  return (
    <div className="guide-rune-tree">
      <div className="guide-rune-tree-title">
        {style ? <img src={runeStyleImageUrl(runes, style.id)} alt="" /> : null}
        <strong>{style?.name ?? 'Rune Tree'}</strong>
      </div>
      <div className="guide-rune-slots">
        {(style?.slots ?? []).map((slot, index) => (
          <div key={`${style?.id}-${index}`} className="guide-rune-slot">
            {slot.runes.map((rune) => {
              const active = selected.has(rune.id);
              return (
                <img
                  key={rune.id}
                  className={active ? 'selected' : ''}
                  src={runeImageUrl(runes, rune.id)}
                  alt={rune.name}
                  title={active ? `${rune.name} selected` : rune.name}
                />
              );
            })}
          </div>
        ))}
      </div>
    </div>
  );
}

function SpellGuideCard({ guide, spells, loading }: { guide?: ChampionGuideResponse; spells?: SummonerSpellData; loading: boolean }) {
  const spellRow = guide?.topSpells[0];
  const spellIds = signatureSpells(spellRow?.spellSignature ?? '');
  return (
    <PanelCard className="guide-card spell-guide-card">
      <PanelTitle title="Summoner Spells" detail={spellRow ? `${spellRow.winRate.toFixed(2)}% WR (${formatNumber(spellRow.games)} matches)` : loading ? 'Loading...' : 'No spell sample yet'} />
      {spellIds.length ? (
        <div className="guide-spell-pair">
          {spellIds.map((spellId) => (
            <img key={spellId} src={summonerSpellImageUrl(spells, spellId)} alt={summonerSpellName(spells, spellId)} title={summonerSpellName(spells, spellId)} />
          ))}
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

function ItemGuideGrid({ rows, items, loading, context }: { rows: AnalyticsItemSlot[]; items?: ItemData; loading: boolean; context: string }) {
  const slotRows = [1, 2, 3, 4, 5, 6].map((slot) => rows.filter((row) => row.itemSlot === slot));
  const coreRows = slotRows.slice(0, 3).map((candidates) => candidates[0]).filter(Boolean) as AnalyticsItemSlot[];
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

function ItemPathSummaryCard({ guide, items, loading }: { guide?: ChampionGuideResponse; items?: ItemData; loading: boolean }) {
  const rows = guide?.topItemPaths ?? [];
  return (
    <PanelCard className="guide-card item-path-summary-card">
      <PanelTitle
        title="Item Paths"
        detail={rows[0] ? `${rows[0].winRate.toFixed(2)}% WR (${formatNumber(rows[0].games)} matches) for the strongest complete path` : loading ? 'Loading...' : 'No complete path sample yet'}
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
      {itemIds.length ? itemIds.map((itemId) => (
        <img key={`${signature}-${itemId}`} src={itemImageUrl(items, String(itemId))} alt={itemName(items, String(itemId))} title={itemName(items, String(itemId))} />
      )) : <em>No items</em>}
    </div>
  );
}

function GuideItemPanel({ title, subtitle, rows, items, loading, linked }: { title: string; subtitle: string; rows: AnalyticsItemSlot[]; items?: ItemData; loading: boolean; linked?: boolean }) {
  return (
    <PanelCard className="guide-card guide-item-panel">
      <PanelTitle title={title} />
      {rows.length ? (
        <div className={linked ? 'guide-item-list linked' : 'guide-item-list'}>
          {rows.map((row, index) => (
            <div key={`${title}-${row.itemSlot}-${row.itemId}`} className="guide-item-option">
              {index > 0 && linked ? <span className="guide-item-arrow">-&gt;</span> : null}
              <img src={itemImageUrl(items, String(row.itemId))} alt={itemName(items, String(row.itemId))} title={itemName(items, String(row.itemId))} />
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
  const summary = guide?.summary;
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

function statPerkLabel(id: number) {
  const labels: Record<number, string> = {
    5001: 'Health',
    5005: 'Attack Speed',
    5007: 'Ability Haste',
    5008: 'Adaptive',
    5010: 'Move Speed',
    5011: 'Health',
    5013: 'Tenacity',
  };
  return labels[id] ?? String(id);
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
