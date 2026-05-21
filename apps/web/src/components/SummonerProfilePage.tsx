import { useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { CalendarDays, CircleAlert, History, LoaderCircle, RadioTower, Shield, Trophy } from 'lucide-react';
import { getLiveGame, getSummonerProfile, resolveAccountAlias } from '../api/client';
import type { AccountAliasMatch, ChampionData, ChampionRecord, ItemData, RankedRecord, RuneData, SummonerProfile, SummonerRecentMatch, SummonerSpellData } from '../api/types';
import { platformLabel } from '../lib/lookup';
import { championByKey, championImageUrl, rankIconUrl, rankLabel } from '../lib/staticData';
import { LiveMatchups } from './LiveMatchups';
import { RoleIcon, roleLabel } from '../lib/roles';

type ProfileSection = 'overview' | 'champions' | 'matches';

const profileSections: { key: ProfileSection; label: string }[] = [
  { key: 'overview', label: 'Overview' },
  { key: 'champions', label: 'Champion Stats' },
  { key: 'matches', label: 'Match History' },
];

type Props = {
  platform?: string;
  gameName?: string;
  tagLine?: string;
  champions?: ChampionData;
  items?: ItemData;
  spells?: SummonerSpellData;
  runes?: RuneData;
  onUseAlias: (alias: AccountAliasMatch) => void;
  onResolvedAlias?: (alias: AccountAliasMatch) => void;
};

export function SummonerProfilePage({
  platform = 'NA1',
  gameName,
  tagLine,
  champions,
  items,
  spells,
  runes,
  onUseAlias,
  onResolvedAlias,
}: Props) {
  const aliasQuery = useQuery({
    queryKey: ['summoner-profile-alias', platform, gameName],
    queryFn: () => resolveAccountAlias(gameName!, platform),
    enabled: Boolean(gameName && !tagLine),
    retry: false,
    staleTime: 30_000,
  });
  const foundAlias = !tagLine && aliasQuery.data?.status === 'found' && aliasQuery.data.gameName && aliasQuery.data.tagLine
    ? {
      puuid: aliasQuery.data.puuid ?? '',
      platform: aliasQuery.data.platform ?? platform,
      gameName: aliasQuery.data.gameName,
      tagLine: aliasQuery.data.tagLine,
    }
    : undefined;
  const resolvedPlatform = tagLine ? platform : foundAlias?.platform ?? platform;
  const resolvedGameName = tagLine ? gameName : foundAlias?.gameName;
  const resolvedTagLine = tagLine || foundAlias?.tagLine;
  const exactRiotId = resolvedGameName && resolvedTagLine ? `${resolvedGameName}#${resolvedTagLine}` : gameName ?? '';
  const liveQuery = useQuery({
    queryKey: ['summoner-profile-live', resolvedPlatform, resolvedGameName, resolvedTagLine],
    queryFn: () => getLiveGame(resolvedGameName!, resolvedTagLine!, resolvedPlatform),
    enabled: Boolean(resolvedGameName && resolvedTagLine),
    retry: false,
    staleTime: 15_000,
  });
  const profileQuery = useQuery({
    queryKey: ['summoner-profile-stored', resolvedPlatform, resolvedGameName, resolvedTagLine],
    queryFn: () => getSummonerProfile(resolvedGameName!, resolvedTagLine!, resolvedPlatform),
    enabled: Boolean(resolvedGameName && resolvedTagLine && !liveQuery.isFetching && !liveQuery.data),
    retry: false,
    staleTime: 60_000,
  });

  useEffect(() => {
    if (foundAlias) {
      onResolvedAlias?.(foundAlias);
    }
  }, [foundAlias, onResolvedAlias]);

  if (liveQuery.data) {
    return (
      <section className="profile-live-shell live-panel has-game">
        <LiveMatchups
          liveGame={liveQuery.data}
          champions={champions}
          items={items}
          spells={spells}
          runes={runes}
        />
      </section>
    );
  }

  const ambiguousMatches = aliasQuery.data?.status === 'ambiguous' ? aliasQuery.data.matches ?? [] : [];
  const liveError = liveQuery.error instanceof Error ? liveQuery.error : undefined;
  const notInGame = Boolean(liveError);
  const showAliasLoading = aliasQuery.isLoading && Boolean(gameName && !tagLine);
  const showLiveLoading = liveQuery.isLoading && Boolean(resolvedGameName && resolvedTagLine);

  return (
    <section className="profile-page">
      <div className="profile-card">
        <div className="profile-card-header">
          <RadioTower size={24} />
          <div>
            <span>Summoner Profile</span>
            <strong>{exactRiotId || 'Search for a Riot ID'}</strong>
          </div>
          <em>{platformLabel(resolvedPlatform)}</em>
        </div>

        {!gameName ? (
          <ProfileMessage
            title="Ready when you are"
            body="Search a champion name to open a champion guide, or search a Riot ID to open this profile page and check for a live match."
          />
        ) : null}

        {showAliasLoading || showLiveLoading ? (
          <ProfileMessage
            icon={<LoaderCircle size={17} className="spin-icon" />}
            title={showAliasLoading ? 'Resolving saved Riot ID' : 'Checking live match'}
            body={showAliasLoading ? `Looking for a unique saved tag for ${gameName}.` : `Checking whether ${exactRiotId} is currently in game.`}
          />
        ) : null}

        {ambiguousMatches.length ? (
          <div className="profile-alias-list">
            <ProfileMessage
              title="More than one saved tag matched"
              body={`Choose the exact Riot ID for ${gameName}, or search with Name#Tag.`}
            />
            <div>
              {ambiguousMatches.map((alias) => (
                <button key={`${alias.platform}:${alias.puuid}:${alias.tagLine}`} type="button" onClick={() => onUseAlias(alias)}>
                  <span>{alias.gameName}</span>
                  <b>#{alias.tagLine}</b>
                  <em>{platformLabel(alias.platform)}</em>
                </button>
              ))}
            </div>
          </div>
        ) : null}

        {gameName && !tagLine && aliasQuery.data?.status === 'not_found' ? (
          <ProfileMessage
            icon={<CircleAlert size={17} />}
            title="Tag needed"
            body={`I do not have a unique saved tag for ${gameName} yet. Search with Name#Tag and I can check the live game directly.`}
          />
        ) : null}

        {notInGame ? (
          <ProfileMessage
            icon={<CircleAlert size={17} />}
            title="Not currently in a live match"
            body={profileLiveError(exactRiotId, liveError?.message ?? '')}
          />
        ) : null}

        {profileQuery.isLoading ? (
          <ProfileMessage
            icon={<LoaderCircle size={17} className="spin-icon" />}
            title="Loading stored profile"
            body={`Checking collected ranked Solo/Duo data for ${exactRiotId}.`}
          />
        ) : null}

        {profileQuery.data ? (
          <StoredProfile profile={profileQuery.data} champions={champions} />
        ) : null}

        {profileQuery.error ? (
          <ProfileMessage
            icon={<CircleAlert size={17} />}
            title="No stored profile yet"
            body="I can check live status for this Riot ID, but I do not have enough stored match data tied to this alias yet. The collector will fill this in when it reaches their matches."
          />
        ) : null}

        {gameName ? (
          <div className="profile-next-panel">
            <span>Profile Scope</span>
            <p>These stats come from WinRift's stored ranked Solo/Duo matches and cached rank snapshots. Live-game lookup still jumps into the match room when the player is currently in game.</p>
          </div>
        ) : null}
      </div>
    </section>
  );
}

function StoredProfile({ profile, champions }: { profile: SummonerProfile; champions?: ChampionData }) {
  const rank = profile.rank;
  const summary = profile.summary;
  const [section, setSection] = useState<ProfileSection>('overview');
  const bestChampion = profile.topChampions[0];
  return (
    <div className="profile-data-stack">
      <div className="profile-snapshot-strip">
        <ProfileSnapshot icon={<Shield size={16} />} label="Solo/Duo" value={rank ? rankLabel(rank) : 'Rank Unknown'} detail={rank ? `${rank.leaguePoints} LP · ${rank.winRate.toFixed(1)}% WR` : 'No cached rank snapshot'} />
        <ProfileSnapshot icon={<Trophy size={16} />} label="Stored Form" value={`${summary.winRate.toFixed(1)}% WR`} detail={`${formatNumber(summary.games)} games · ${summary.kda ? summary.kda.toFixed(2) : '--'} KDA`} />
        <ProfileSnapshot icon={<Trophy size={16} />} label="Main Champion" value={bestChampion ? championName(champions, bestChampion.championId) : 'No Sample'} detail={bestChampion ? `${formatNumber(bestChampion.games)} games · ${bestChampion.winRate.toFixed(1)}% WR` : 'Collector has not reached champion history'} />
        <ProfileSnapshot icon={<CalendarDays size={16} />} label="Last Stored" value={formatProfileDate(summary.lastSeen)} detail={`${formatProfileDate(summary.firstSeen)} first seen`} />
      </div>

      <div className="profile-section-tabs" aria-label="Summoner profile sections">
        {profileSections.map((candidate) => (
          <button className={section === candidate.key ? 'selected' : ''} key={candidate.key} onClick={() => setSection(candidate.key)} type="button">
            {candidate.label}
          </button>
        ))}
      </div>

      {section === 'overview' ? (
        <div className="profile-data-grid">
          <RankedPanel rank={rank} />
          <FormPanel summary={summary} />
          <DataWindowPanel summary={summary} />
          <ChampionComfortPanel champions={champions} records={profile.topChampions.slice(0, 6)} title="Champion Comfort" />
          <RecentMatchesPanel champions={champions} matches={profile.recentMatches.slice(0, 8)} title="Recent Stored Matches" />
        </div>
      ) : null}

      {section === 'champions' ? (
        <ChampionComfortPanel champions={champions} records={profile.topChampions} title="Champion Stats" wide />
      ) : null}

      {section === 'matches' ? (
        <RecentMatchesPanel champions={champions} matches={profile.recentMatches} title="Match History" wide />
      ) : null}
    </div>
  );
}

function ProfileSnapshot({ detail, icon, label, value }: { detail: string; icon: ReactNode; label: string; value: string }) {
  return (
    <div className="profile-snapshot-card">
      {icon}
      <div>
        <span>{label}</span>
        <strong>{value}</strong>
        <em>{detail}</em>
      </div>
    </div>
  );
}

function RankedPanel({ rank }: { rank?: RankedRecord }) {
  return (
    <section className="profile-panel profile-rank-panel">
      <PanelHeading icon={<Shield size={16} />} title="Ranked Solo/Duo" />
      <div className="profile-rank-read">
        <img src={rankIconUrl(rank)} alt={rank ? rankLabel(rank) : 'Rank unavailable'} />
        <div>
          <strong>{rank ? rankLabel(rank) : 'Rank Unknown'}</strong>
          <span>{rank ? `${rank.leaguePoints} LP · ${formatNumber(rank.totalGames)} ranked games` : 'No cached rank snapshot yet'}</span>
        </div>
      </div>
      {rank ? (
        <div className="profile-metric-row">
          <ProfileMetric label="Winrate" value={`${rank.winRate.toFixed(1)}%`} />
          <ProfileMetric label="Wins" value={formatNumber(rank.wins)} />
          <ProfileMetric label="Losses" value={formatNumber(rank.losses)} />
        </div>
      ) : null}
    </section>
  );
}

function FormPanel({ summary }: { summary: SummonerProfile['summary'] }) {
  return (
    <section className="profile-panel">
      <PanelHeading icon={<Trophy size={16} />} title="Stored Match Form" />
      <div className="profile-metric-row">
        <ProfileMetric label="Games" value={formatNumber(summary.games)} />
        <ProfileMetric label="Winrate" value={`${summary.winRate.toFixed(1)}%`} />
        <ProfileMetric label="KDA" value={summary.kda ? summary.kda.toFixed(2) : '--'} />
      </div>
      <div className="profile-kda-line">
        {summary.avgKills.toFixed(1)} / {summary.avgDeaths.toFixed(1)} / {summary.avgAssists.toFixed(1)} average across collected ranked games
      </div>
    </section>
  );
}

function DataWindowPanel({ summary }: { summary: SummonerProfile['summary'] }) {
  return (
    <section className="profile-panel profile-window-panel">
      <PanelHeading icon={<CalendarDays size={16} />} title="Stored Data Window" />
      <div className="profile-metric-row">
        <ProfileMetric label="First Seen" value={formatProfileDate(summary.firstSeen)} />
        <ProfileMetric label="Last Seen" value={formatProfileDate(summary.lastSeen)} />
        <ProfileMetric label="Sample" value={`${formatNumber(summary.games)} games`} />
      </div>
      <div className="profile-kda-line">
        Profile summaries are compact read-model rows refreshed from retained ranked Solo/Duo matches.
      </div>
    </section>
  );
}

function ChampionComfortPanel({ champions, records, title, wide = false }: { champions?: ChampionData; records: ChampionRecord[]; title: string; wide?: boolean }) {
  return (
    <section className={`profile-panel profile-wide-panel${wide ? ' profile-tab-panel' : ''}`}>
      <PanelHeading icon={<Trophy size={16} />} title={title} />
      {records.length ? (
        <div className="profile-champion-list">
          {records.map((record) => (
            <ChampionComfortRow key={record.championId} record={record} champions={champions} />
          ))}
        </div>
      ) : (
        <p className="profile-empty-text">No stored champion games for this profile yet.</p>
      )}
    </section>
  );
}

function RecentMatchesPanel({ champions, matches, title, wide = false }: { champions?: ChampionData; matches: SummonerRecentMatch[]; title: string; wide?: boolean }) {
  return (
    <section className={`profile-panel profile-wide-panel${wide ? ' profile-tab-panel' : ''}`}>
      <PanelHeading icon={<History size={16} />} title={title} />
      {matches.length ? (
        <div className="profile-match-list">
          {matches.map((match) => (
            <RecentMatchRow key={`${match.matchId}:${match.championId}`} match={match} champions={champions} />
          ))}
        </div>
      ) : (
        <p className="profile-empty-text">Recent matches will appear once stored games are attached to this alias.</p>
      )}
    </section>
  );
}

function PanelHeading({ icon, title }: { icon: ReactNode; title: string }) {
  return (
    <div className="profile-panel-heading">
      {icon}
      <span>{title}</span>
    </div>
  );
}

function ProfileMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="profile-metric">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function ChampionComfortRow({ record, champions }: { record: ChampionRecord; champions?: ChampionData }) {
  const champion = championByKey(champions, record.championId);
  return (
    <div className="profile-champion-row">
      <img src={championImageUrl(champions, record.championId)} alt={champion?.name ?? String(record.championId)} />
      <div>
        <strong>{champion?.name ?? `Champion ${record.championId}`}</strong>
        <span>{record.avgKills.toFixed(1)} / {record.avgDeaths.toFixed(1)} / {record.avgAssists.toFixed(1)} average</span>
      </div>
      <div className="profile-row-stats">
        <ProfileMiniStat label="WR" value={`${record.winRate.toFixed(1)}%`} />
        <ProfileMiniStat label="KDA" value={record.kda.toFixed(2)} />
        <ProfileMiniStat label="Games" value={formatNumber(record.games)} />
      </div>
    </div>
  );
}

function RecentMatchRow({ match, champions }: { match: SummonerRecentMatch; champions?: ChampionData }) {
  const champion = championByKey(champions, match.championId);
  return (
    <div className={`profile-match-row ${match.win ? 'win' : 'loss'}`}>
      <img src={championImageUrl(champions, match.championId)} alt={champion?.name ?? String(match.championId)} />
      <div>
        <strong>{match.win ? 'Win' : 'Loss'} · {champion?.name ?? `Champion ${match.championId}`}</strong>
        <span><RoleIcon role={match.role} /> {roleLabel(match.role)} · {match.kills}/{match.deaths}/{match.assists} · Patch {match.patch} · {formatGameDate(match.gameStartTimestamp)} · {formatDuration(match.durationSeconds)}</span>
      </div>
      <div className="profile-row-stats">
        <ProfileMiniStat label="Result" value={match.win ? 'Win' : 'Loss'} tone={match.win ? 'win' : 'loss'} />
        <ProfileMiniStat label="KDA" value={`${match.kills}/${match.deaths}/${match.assists}`} />
        <ProfileMiniStat label="Role" value={roleLabel(match.role)} />
      </div>
    </div>
  );
}

function ProfileMiniStat({ label, tone, value }: { label: string; tone?: 'win' | 'loss'; value: string }) {
  return (
    <span className={`profile-mini-stat${tone ? ` ${tone}` : ''}`}>
      <em>{label}</em>
      <b>{value}</b>
    </span>
  );
}

function ProfileMessage({ icon, title, body }: { icon?: ReactNode; title: string; body: string }) {
  return (
    <div className="profile-message">
      {icon}
      <div>
        <strong>{title}</strong>
        <span>{body}</span>
      </div>
    </div>
  );
}

function profileLiveError(riotId: string, message: string) {
  const normalized = message.toLowerCase();
  if (normalized.includes('not currently in a live game') || normalized.includes('riot id not found')) {
    return `Summoner '${riotId.trim()}' is not currently in a live match`;
  }
  return message;
}

function formatNumber(value: number) {
  return new Intl.NumberFormat('en-US').format(value);
}

function championName(champions: ChampionData | undefined, championId: number) {
  return championByKey(champions, championId)?.name ?? `Champion ${championId}`;
}

function formatProfileDate(value?: string) {
  if (!value) return '--';
  const date = new Date(value);
  if (Number.isNaN(date.getTime()) || date.getFullYear() <= 1970) return '--';
  return new Intl.DateTimeFormat('en-US', { month: 'short', day: 'numeric' }).format(date);
}

function formatGameDate(timestamp: number) {
  if (!timestamp) return 'unknown date';
  const date = new Date(timestamp);
  if (Number.isNaN(date.getTime())) return 'unknown date';
  return new Intl.DateTimeFormat('en-US', { month: 'short', day: 'numeric' }).format(date);
}

function formatDuration(seconds: number) {
  if (!seconds) return '--';
  const minutes = Math.floor(seconds / 60);
  const remaining = seconds % 60;
  return `${minutes}:${String(remaining).padStart(2, '0')}`;
}
