import { GripVertical } from 'lucide-react';
import type { DragEvent } from 'react';
import type { ChampionData, ChampionRecord, LiveParticipant, RankedRecord, RuneData, SummonerSpellData } from '../../api/types';
import { RoleIcon, roleLabel } from '../../lib/roles';
import {
  championByKey,
  championImageUrl,
  championSplashUrl,
  rankIconUrl,
  rankLabel,
  runeImageUrl,
  runeName,
  runeStyleName,
  summonerSpellImageUrl,
  summonerSpellName,
} from '../../lib/staticData';
import { StatChip } from '../ui/MetricTile';
import { StatusChip } from '../ui/StatusChip';
import type { TeamSide } from './types';

export function LiveChampionCard({
  participant,
  index,
  role,
  side,
  champions,
  spells,
  runes,
  dragging,
  dropTarget,
  onDragStart,
  onDragOver,
  onDrop,
  onDragEnd,
  mobileActive,
}: {
  participant: LiveParticipant;
  index: number;
  role: string;
  side: TeamSide;
  champions?: ChampionData;
  spells?: SummonerSpellData;
  runes?: RuneData;
  dragging: boolean;
  dropTarget: boolean;
  onDragStart: () => void;
  onDragOver: (event: DragEvent<HTMLElement>) => void;
  onDrop: (event: DragEvent<HTMLElement>) => void;
  onDragEnd: () => void;
  mobileActive: boolean;
}) {
  const champion = championByKey(champions, participant.championId);
  const championName = champion?.name ?? String(participant.championId);
  const championUrl = championImageUrl(champions, participant.championId);
  const championBackdropUrl = championSplashUrl(champions, participant.championId);
  const keystoneId = participant.perks?.perkIds?.[0];
  const secondaryStyleId = participant.perks?.perkSubStyle;
  const playerName = participant.riotId || participant.summonerName || 'Unknown player';
  const comfortFlags = comfortFlagsForParticipant(participant);

  return (
    <article
      className={`player-card ${side}-style${dragging ? ' dragging' : ''}${dropTarget ? ' drop-target' : ''}${mobileActive ? ' mobile-active' : ' mobile-hidden'}`}
      draggable
      onDragStart={(event) => {
        event.dataTransfer.effectAllowed = 'move';
        event.dataTransfer.setData('text/plain', `${side}:${index}`);
        onDragStart();
      }}
      onDragOver={onDragOver}
      onDrop={onDrop}
      onDragEnd={onDragEnd}
      title="Drag to reorder matchup slots"
    >
      {championBackdropUrl ? <img className="player-card-backdrop" src={championBackdropUrl} alt="" aria-hidden="true" /> : null}
      <GripVertical className="card-drag-handle" size={16} aria-hidden="true" />
      <div className="card-topline">
        {championUrl ? <img className="profile-picture" src={championUrl} alt={championName} /> : <div className="profile-picture profile-fallback">{participant.championId}</div>}
        <img className="ranked-card-icon" src={rankIconUrl(participant.rank)} alt={participant.rank ? rankLabel(participant.rank) : 'Rank unavailable'} />
        <div className="player-title">
          <div className="player-role-chip"><RoleIcon role={role} /><span>{roleLabel(role)}</span></div>
          <div className="summoner-name" title={playerName}>{playerName}</div>
          <div className="champion-name">{championName}</div>
        </div>
      </div>
      {comfortFlags.length > 0 ? (
        <div className="card-context-row">
          {comfortFlags.map((flag) => (
            <StatusChip className="comfort-flag" key={flag.label} tone={flag.tone}>{flag.label}</StatusChip>
          ))}
        </div>
      ) : null}
      <div className="player-stat-columns">
        <ChampionRecordBlock stats={participant.championStats} />
        <RankRecord rank={participant.rank} />
      </div>
      <div className="card-loadout">
        <div className="loadout-group">
          <span>Spells</span>
          <div className="summoner-spells">
            {[participant.spell1Id, participant.spell2Id].map((spellId) => {
              const imageUrl = summonerSpellImageUrl(spells, spellId);
              const name = summonerSpellName(spells, spellId);
              return imageUrl ? <img key={spellId} src={imageUrl} alt={name} title={name} /> : <span className="spell-pill" key={spellId}>{spellId}</span>;
            })}
          </div>
        </div>
        <div className="loadout-group">
          <span>Runes</span>
          <div className="runes">
            <RuneIcon runes={runes} runeId={keystoneId} />
            <RuneStyleIcon runes={runes} styleId={secondaryStyleId} />
          </div>
        </div>
      </div>
    </article>
  );
}

function RankRecord({ rank }: { rank?: RankedRecord }) {
  if (!rank) {
    return (
      <div className="rank-info rank-missing">
        <div className="stat-block-title">Ranked</div>
        <div className="stat-chip-grid">
          <StatChip label="Rank" value="Unavailable" primary />
          <StatChip label="Winrate" value="--" />
          <StatChip label="Games" value="--" />
        </div>
      </div>
    );
  }
  return (
    <div className={`rank-info${rank.rankAvailable === false ? ' rank-unranked' : ''}`}>
      <div className="stat-block-title">Ranked</div>
      <div className="stat-chip-grid">
        <StatChip label="Rank" value={rankLabel(rank)} primary />
        <StatChip label="Winrate" value={`${rank.winRate.toFixed(1)}%`} />
        <StatChip label="Games" value={String(rank.totalGames)} />
      </div>
    </div>
  );
}

function ChampionRecordBlock({ stats }: { stats?: ChampionRecord }) {
  if (!stats) {
    return (
      <div className="champion-performance stats-missing">
        <div className="stat-block-title">Champion</div>
        <div className="stat-chip-grid">
          <StatChip label="Games" value="0" primary />
          <StatChip label="Champ WR" value="--" />
          <StatChip label="KDA" value="--" />
        </div>
      </div>
    );
  }
  return (
    <div className="champion-performance">
      <div className="stat-block-title">Champion</div>
      <div className="stat-chip-grid">
        <StatChip label="Games" value={String(stats.games)} primary />
        <StatChip label="Champ WR" value={`${stats.winRate.toFixed(1)}%`} />
        <StatChip label="KDA" value={stats.kda.toFixed(2)} />
        <StatChip label="Avg K/D/A" value={`${stats.avgKills.toFixed(1)} / ${stats.avgDeaths.toFixed(1)} / ${stats.avgAssists.toFixed(1)}`} wide />
      </div>
    </div>
  );
}

function RuneIcon({ runes, runeId }: { runes?: RuneData; runeId?: number }) {
  const imageUrl = runeImageUrl(runes, runeId);
  const name = runeName(runes, runeId);
  return imageUrl ? <img src={imageUrl} alt={name} title={name} /> : <span className="rune-fallback">{runeId ?? '?'}</span>;
}

function RuneStyleIcon({ runes, styleId }: { runes?: RuneData; styleId?: number }) {
  const styleName = runeStyleName(runes, styleId);
  const style = runes?.data.find((candidate) => candidate.id === styleId);
  return style?.icon ? <img src={`https://ddragon.leagueoflegends.com/cdn/img/${style.icon}`} alt={styleName} title={styleName} /> : <span className="rune-fallback">{styleId ?? '?'}</span>;
}

function comfortFlagsForParticipant(participant: LiveParticipant) {
  const flags: { label: string; tone: string }[] = [];
  const stats = participant.championStats;
  if (!stats || stats.games <= 0) {
    flags.push({ label: 'No champ sample', tone: 'thin' });
  } else if (stats.games < 5) {
    flags.push({ label: 'Low champ sample', tone: 'thin' });
  } else if (stats.games >= 20 && stats.winRate >= 55) {
    flags.push({ label: 'Champion comfort', tone: 'good' });
  } else if (stats.games >= 20 && stats.winRate <= 45) {
    flags.push({ label: 'Rough champ sample', tone: 'warn' });
  }

  const rank = participant.rank;
  if (!rank || rank.rankAvailable === false) {
    flags.push({ label: 'Rank pending', tone: 'thin' });
  } else if (rank.totalGames >= 30 && rank.winRate >= 55) {
    flags.push({ label: 'Strong ranked form', tone: 'good' });
  }

  return flags.slice(0, 2);
}
