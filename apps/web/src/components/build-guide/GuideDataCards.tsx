import type { Champion, ChampionData, ChampionGuideBuildVariant, ChampionGuideResponse, RuneData, RuneStyle, SummonerSpellData } from '../../api/types';
import {
  championAbilityImageUrl,
  championByKey,
  championImageUrl,
  parseRuneSignature,
  runeImageUrl,
  runeStyleImageUrl,
  signatureSpells,
  summonerSpellImageUrl,
  summonerSpellName,
} from '../../lib/staticData';
import { EmptyState, PanelCard, PanelTitle } from '../ui/Panel';
import { StatShardGrid } from '../ui/StatShardDisplay';

export function RuneGuideCard({ guide, variant, runes, loading }: { guide?: ChampionGuideResponse; variant?: ChampionGuideBuildVariant; runes?: RuneData; loading: boolean }) {
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

export function SpellGuideCard({ guide, variant, spells, loading }: { guide?: ChampionGuideResponse; variant?: ChampionGuideBuildVariant; spells?: SummonerSpellData; loading: boolean }) {
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

export function MatchupStrip({ title, subtitle, rows, champions, tone, loading }: { title: string; subtitle: string; rows: { opponentChampionId: number; winRate: number; games: number }[]; champions?: ChampionData; tone: 'good' | 'bad'; loading: boolean }) {
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

export function SkillGuideCard({ guide, variant, champion, champions, loading }: { guide?: ChampionGuideResponse; variant?: ChampionGuideBuildVariant; champion?: Champion; champions?: ChampionData; loading: boolean }) {
  const spells = champion?.spells ?? [];
  const skillOrder = variant?.skillOrderSignature ? variantSkillOrderRow(variant) : guide?.topSkillOrders?.[0];
  const priority = skillPriority(skillOrder?.skillOrderSignature ?? '');
  const detail = skillOrder
    ? `${skillOrder.winRate.toFixed(2)}% WR (${formatNumber(skillOrder.games)} matches)${variant?.skillOrderSignature ? ' for this build' : ''}`
    : loading ? 'Loading...' : 'No skill sample yet';
  return (
    <PanelCard className="guide-card skill-priority-card">
      <PanelTitle title="Skill Priority" detail={detail} />
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
        <p>{variant?.skillOrderSignature ? 'This priority is filtered to the selected build family.' : 'Derived from Match-V5 timeline skill-level events in collected ranked games.'}</p>
      ) : (
        <p>{loading ? 'Checking collected timelines.' : 'Skill order will appear after guide analytics refreshes for this champion.'}</p>
      )}
    </PanelCard>
  );
}

export function SkillPathCard({ guide, variant, championName, loading }: { guide?: ChampionGuideResponse; variant?: ChampionGuideBuildVariant; championName: string; loading: boolean }) {
  const skillOrder = variant?.skillOrderSignature ? variantSkillOrderRow(variant) : guide?.topSkillOrders?.[0];
  const slots = skillSlots(skillOrder?.skillOrderSignature ?? '');
  return (
    <PanelCard className="guide-card skill-path-card">
      <PanelTitle title="Skill Path" detail={skillOrder ? `${formatNumber(skillOrder.games)} matching paths${variant?.skillOrderSignature ? ' for this build' : ''}` : loading ? 'Loading...' : 'No path sample yet'} />
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
      <p>{slots.length ? `${championName} skill order is shown from the most common collected path${variant?.skillOrderSignature ? ' for this build family' : ''}.` : 'This panel stays empty until real timeline data exists.'}</p>
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

function variantSkillOrderRow(variant: ChampionGuideBuildVariant) {
  return {
    skillOrderSignature: variant.skillOrderSignature ?? '',
    wins: variant.skillOrderWins ?? 0,
    games: variant.skillOrderGames ?? 0,
    winRate: variant.skillOrderWinRate ?? 0,
    confidence: variant.skillOrderConfidence ?? 0,
  };
}

function skillSlots(signature: string) {
  return signature
    .split('-')
    .map((part) => Number(part))
    .filter((slot) => slot >= 1 && slot <= 4);
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

function formatNumber(value: number) {
  return new Intl.NumberFormat('en-US').format(value);
}
