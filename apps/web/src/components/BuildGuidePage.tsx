import { CircleSlash, Database, Filter, Search, Shield } from 'lucide-react';
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
  items?: ItemData;
  spells?: SummonerSpellData;
  runes?: RuneData;
};

export function BuildGuidePage({ champions, items, spells, runes }: Props) {
  const championsByName = useMemo(() => championList(champions), [champions]);
  const defaultChampionId = useMemo(() => {
    const wukong = championsByName.find((champion) => champion.id === 'MonkeyKing');
    return Number(wukong?.key ?? championsByName[0]?.key ?? 62);
  }, [championsByName]);
  const [championId, setChampionId] = useState(defaultChampionId);
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
    if (defaultChampionId && !championByKey(champions, championId)) {
      setChampionId(defaultChampionId);
    }
  }, [champions, championId, defaultChampionId]);

  const guide = guideQuery.data;
  const itemSlots = itemSlotsQuery.data?.results ?? [];
  const splash = championSplashUrl(champions, championId);
  const rankLabel = ranks.find((candidate) => candidate.value === rankBucket)?.label ?? 'All Ranks';
  const titleRole = roles.find((candidate) => candidate.value === role)?.label ?? role;
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
              <span>{champion?.name ?? 'Champion'}</span> <em>{titleRole} patterns</em>
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
        onChampionChange={setChampionId}
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
        <SkillGuideCard champion={champion} champions={champions} />
        <SkillPathPlaceholder championName={champion?.name ?? 'this champion'} />
      </div>

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
      <label className="guide-select-control">
        <span>Champion</span>
        <select value={championId} onChange={(event) => onChampionChange(Number(event.target.value))}>
          {champions.map((champion) => {
            const games = coverage.get(Number(champion.key))?.games ?? 0;
            return (
              <option key={champion.key} value={champion.key}>
                {champion.name}{games ? ` (${formatNumber(games)})` : ''}
              </option>
            );
          })}
        </select>
      </label>
      <div className="guide-role-tabs" aria-label="Role">
        {roles.map((candidate) => (
          <button key={candidate.value} className={candidate.value === role ? 'selected' : ''} onClick={() => onRoleChange(candidate.value)} type="button">
            {candidate.label}
          </button>
        ))}
      </div>
      <label className="guide-select-control compact">
        <span>Rank</span>
        <select value={rankBucket} onChange={(event) => onRankChange(event.target.value)}>
          {ranks.map((candidate) => (
            <option key={candidate.value || 'ALL'} value={candidate.value}>{candidate.label}</option>
          ))}
        </select>
      </label>
      <label className="guide-select-control matchup">
        <Search size={15} />
        <select value={opponentChampionId} onChange={(event) => onOpponentChange(Number(event.target.value))}>
          <option value={0}>vs. Champion...</option>
          {champions.map((champion) => (
            <option key={champion.key} value={champion.key}>{champion.name}</option>
          ))}
        </select>
      </label>
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
      <GuideCoverageStat label="Champions with data" value={loading ? '...' : formatNumber(championCount)} />
      <GuideCoverageStat label={`${role} games indexed`} value={loading ? '...' : formatNumber(totalGames)} />
      <GuideCoverageStat label="Selected champion" value={loading ? '...' : formatNumber(selectedGames)} />
      <GuideCoverageStat label="Scope" value={`${patch || 'all patches'} / ${rankLabel}`} />
    </div>
  );
}

function GuideCoverageStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="guide-coverage-stat">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function GuideStats({ guide, loading }: { guide?: ChampionGuideResponse; loading: boolean }) {
  const summary = guide?.summary;
  return (
    <div className="guide-stat-strip">
      <GuideStat label="Tier" value={loading ? '...' : guideTier(guide)} tone="tier" />
      <GuideStat label="Win Rate" value={summary?.games ? `${summary.winRate.toFixed(2)}%` : '--'} />
      <GuideStat label="Rank" value={summary?.roleRank ? `${summary.roleRank} / ${summary.roleRankTotal}` : '--'} />
      <GuideStat label="Pick Rate" value={summary?.games ? `${summary.pickRate.toFixed(2)}%` : '--'} />
      <GuideStat label="Confidence" value={summary?.games ? `${summary.confidence.toFixed(1)}%` : '--'} />
      <GuideStat label="Matches" value={formatNumber(summary?.games ?? 0)} />
    </div>
  );
}

function GuideStat({ label, value, tone }: { label: string; value: string; tone?: string }) {
  return (
    <div className={`guide-stat ${tone ?? ''}`}>
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function RuneGuideCard({ guide, runes, loading }: { guide?: ChampionGuideResponse; runes?: RuneData; loading: boolean }) {
  const runeRow = guide?.topRunes[0];
  const parsed = parseRuneSignature(runeRow?.runeSignature ?? '');
  const primary = runes?.data.find((style) => style.id === parsed.primaryStyleId);
  const secondary = runes?.data.find((style) => style.id === parsed.secondaryStyleId);
  return (
    <article className="guide-card rune-guide-card">
      <GuideSectionTitle title="Runes" detail={runeRow ? `${runeRow.winRate.toFixed(2)}% WR (${formatNumber(runeRow.games)} matches)` : loading ? 'Loading...' : 'No rune sample yet'} />
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
      ) : <GuideEmpty message="Rune pages will appear once this champion has enough collected games." />}
    </article>
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
    <article className="guide-card spell-guide-card">
      <GuideSectionTitle title="Summoner Spells" detail={spellRow ? `${spellRow.winRate.toFixed(2)}% WR (${formatNumber(spellRow.games)} matches)` : loading ? 'Loading...' : 'No spell sample yet'} />
      {spellIds.length ? (
        <div className="guide-spell-pair">
          {spellIds.map((spellId) => (
            <img key={spellId} src={summonerSpellImageUrl(spells, spellId)} alt={summonerSpellName(spells, spellId)} title={summonerSpellName(spells, spellId)} />
          ))}
        </div>
      ) : <GuideEmpty message="Summoner spell samples will appear after collection." />}
    </article>
  );
}

function MatchupStrip({ title, subtitle, rows, champions, tone, loading }: { title: string; subtitle: string; rows: { opponentChampionId: number; winRate: number; games: number }[]; champions?: ChampionData; tone: 'good' | 'bad'; loading: boolean }) {
  return (
    <section className="guide-card matchup-strip">
      <GuideSectionTitle title={title} detail={subtitle} />
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
        }) : <GuideEmpty message={loading ? 'Loading matchups...' : 'No matchup sample yet for this filter.'} />}
      </div>
    </section>
  );
}

function SkillGuideCard({ champion, champions }: { champion?: Champion; champions?: ChampionData }) {
  const spells = champion?.spells ?? [];
  return (
    <article className="guide-card skill-priority-card">
      <GuideSectionTitle title="Skill Priority" detail="Ability stats are not normalized yet" />
      <div className="skill-priority-icons">
        {spells.slice(0, 3).map((ability, index) => (
          <span key={ability.id}>
            <img src={championAbilityImageUrl(champions, ability.image?.full, 'spell')} alt={ability.name} title={ability.name} />
            <b>{['Q', 'W', 'E'][index]}</b>
          </span>
        ))}
      </div>
      <p>We can add this once timeline skill-level events are normalized into a summary table.</p>
    </article>
  );
}

function SkillPathPlaceholder({ championName }: { championName: string }) {
  return (
    <article className="guide-card skill-path-card">
      <GuideSectionTitle title="Skill Path" detail="Future read model" />
      <div className="skill-path-placeholder-grid">
        {['Q', 'W', 'E', 'R'].map((spell) => (
          <div key={spell}>
            <b>{spell}</b>
            {Array.from({ length: 18 }, (_, index) => <span key={index} className={index % 5 === 0 ? 'lit' : ''}>{index % 5 === 0 ? index + 1 : ''}</span>)}
          </div>
        ))}
      </div>
      <p>{championName} skill-order analytics are intentionally marked as pending, not guessed.</p>
    </article>
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

function GuideItemPanel({ title, subtitle, rows, items, loading, linked }: { title: string; subtitle: string; rows: AnalyticsItemSlot[]; items?: ItemData; loading: boolean; linked?: boolean }) {
  return (
    <article className="guide-card guide-item-panel">
      <GuideSectionTitle title={title} detail="" />
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
      ) : <GuideEmpty message={loading ? 'Loading items...' : 'No item sample yet.'} />}
      <small>{subtitle}</small>
    </article>
  );
}

function RoleQuestCard() {
  return (
    <article className="guide-card guide-role-quest">
      <GuideSectionTitle title="Role Quest" detail="Jungle context" />
      <p>Jungle builds include starter jungle items when they appear in the collected purchase path. This keeps jungle-specific first-slot data separate from lane builds.</p>
    </article>
  );
}

function GuideMiniNote({ title, body }: { title: string; body: string }) {
  return (
    <article className="guide-card guide-mini-note">
      <GuideSectionTitle title={title} detail="" />
      <p>{body}</p>
    </article>
  );
}

function GuideSectionTitle({ title, detail }: { title: string; detail: string }) {
  return (
    <header className="guide-section-title">
      <span>{title}</span>
      {detail ? <em>{detail}</em> : null}
    </header>
  );
}

function GuideEmpty({ message }: { message: string }) {
  return (
    <div className="guide-empty">
      <CircleSlash size={18} />
      <span>{message}</span>
    </div>
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
