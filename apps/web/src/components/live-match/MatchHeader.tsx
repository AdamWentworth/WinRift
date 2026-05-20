import type { LiveGame, LiveParticipant } from '../../api/types';
import type { TeamSide } from './types';

export function MatchHeader({
  liveGame,
  now,
  patch,
  searchedParticipant,
  yourSide,
  blueTeam,
  redTeam,
}: {
  liveGame: LiveGame;
  now: number;
  patch?: string;
  searchedParticipant?: LiveParticipant;
  yourSide: TeamSide;
  blueTeam: LiveParticipant[];
  redTeam: LiveParticipant[];
}) {
  const blueAverage = averageRankLabel(blueTeam);
  const redAverage = averageRankLabel(redTeam);
  const searchedName = searchedParticipant?.riotId || searchedParticipant?.summonerName || 'Live player';
  return (
    <header className={`match-header ${yourSide}-side`}>
      <div className="match-header-main">
        <div className="match-kicker">{queueLabel(liveGame.gameQueueConfigId)} · {liveGame.platform}</div>
        <h2>{searchedName}</h2>
        <div className="match-subline">
          <span>{liveGame.gameMode}</span>
          <span>{yourSide === 'blue' ? 'Blue side' : 'Red side'}</span>
          {patch ? <span>Patch {patch}</span> : null}
        </div>
      </div>
      <div className="match-header-stats" aria-label="Live match context">
        <HeaderStat label="Clock" value={matchClock(liveGame.gameStartTime, now)} />
        <HeaderStat label="Blue Avg" value={blueAverage.label} detail={blueAverage.detail} tone="blue" />
        <HeaderStat label="Red Avg" value={redAverage.label} detail={redAverage.detail} tone="red" />
      </div>
    </header>
  );
}

function HeaderStat({ label, value, detail, tone }: { label: string; value: string; detail?: string; tone?: TeamSide }) {
  return (
    <div className={`match-header-stat${tone ? ` ${tone}` : ''}`}>
      <span>{label}</span>
      <strong>{value}</strong>
      {detail ? <em>{detail}</em> : null}
    </div>
  );
}

function matchClock(gameStartTime: number, now: number) {
  if (!gameStartTime) return '--:--';
  const elapsedSeconds = Math.max(0, Math.floor((now - gameStartTime) / 1000));
  const minutes = Math.floor(elapsedSeconds / 60);
  const seconds = elapsedSeconds % 60;
  return `${minutes}:${seconds.toString().padStart(2, '0')}`;
}

function averageRankLabel(team: LiveParticipant[]) {
  const values = team.map(rankValue).filter((value): value is number => value !== undefined);
  if (!values.length) {
    return { label: 'Unknown', detail: `0/${team.length || 5} ranked` };
  }
  const average = values.reduce((sum, value) => sum + value, 0) / values.length;
  return {
    label: labelForRankValue(average),
    detail: `${values.length}/${team.length || 5} ranked`,
  };
}

const rankTierValues: Record<string, number> = {
  IRON: 0,
  BRONZE: 4,
  SILVER: 8,
  GOLD: 12,
  PLATINUM: 16,
  EMERALD: 20,
  DIAMOND: 24,
  MASTER: 28,
  GRANDMASTER: 30,
  CHALLENGER: 32,
};

const rankDivisionValues: Record<string, number> = {
  IV: 0,
  III: 1,
  II: 2,
  I: 3,
};

function rankValue(participant: LiveParticipant) {
  const rank = participant.rank;
  if (!rank || rank.rankAvailable === false || !rank.tier) return undefined;
  const tierValue = rankTierValues[rank.tier.toUpperCase()];
  if (tierValue === undefined) return undefined;
  if (tierValue >= rankTierValues.MASTER) {
    return tierValue + Math.min(1.9, Math.max(0, rank.leaguePoints) / 500);
  }
  const division = rank.division || rank.rank || 'IV';
  return tierValue + (rankDivisionValues[division.toUpperCase()] ?? 0) + Math.min(0.99, Math.max(0, rank.leaguePoints) / 100);
}

function labelForRankValue(value: number) {
  if (value >= rankTierValues.CHALLENGER) return 'Challenger';
  if (value >= rankTierValues.GRANDMASTER) return 'Grandmaster';
  if (value >= rankTierValues.MASTER) return 'Master';
  const tiers = ['IRON', 'BRONZE', 'SILVER', 'GOLD', 'PLATINUM', 'EMERALD', 'DIAMOND'];
  const tierIndex = Math.max(0, Math.min(tiers.length - 1, Math.floor(value / 4)));
  const divisionOffset = Math.max(0, Math.min(3, Math.round(value - tierIndex * 4)));
  const division = ['IV', 'III', 'II', 'I'][divisionOffset];
  return `${titleCase(tiers[tierIndex])} ${division}`;
}

function titleCase(value: string) {
  return value.charAt(0).toUpperCase() + value.slice(1).toLowerCase();
}

function queueLabel(queueId: number) {
  if (queueId === 420) return 'Ranked Solo/Duo';
  if (queueId === 440) return 'Ranked Flex';
  if (queueId === 400) return 'Normal Draft';
  if (queueId === 430) return 'Normal Blind';
  return `Queue ${queueId}`;
}
