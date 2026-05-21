import { useEffect } from 'react';
import type { ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { CircleAlert, History, LoaderCircle, RadioTower, Shield, Trophy } from 'lucide-react';
import { getLiveGame, getSummonerProfile, resolveAccountAlias } from '../api/client';
import type { AccountAliasMatch, ChampionData, ChampionRecord, ItemData, RankedRecord, RuneData, SummonerProfile, SummonerRecentMatch, SummonerSpellData } from '../api/types';
import { platformLabel } from '../lib/lookup';
import { championByKey, championImageUrl, rankIconUrl, rankLabel } from '../lib/staticData';
import { LiveMatchups } from './LiveMatchups';
import { RoleIcon, roleLabel } from '../lib/roles';

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
  return (
    <div className="profile-data-grid">
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

      <section className="profile-panel profile-wide-panel">
        <PanelHeading icon={<Trophy size={16} />} title="Champion Comfort" />
        {profile.topChampions.length ? (
          <div className="profile-champion-list">
            {profile.topChampions.map((record) => (
              <ChampionComfortRow key={record.championId} record={record} champions={champions} />
            ))}
          </div>
        ) : (
          <p className="profile-empty-text">No stored champion games for this profile yet.</p>
        )}
      </section>

      <section className="profile-panel profile-wide-panel">
        <PanelHeading icon={<History size={16} />} title="Recent Stored Matches" />
        {profile.recentMatches.length ? (
          <div className="profile-match-list">
            {profile.recentMatches.map((match) => (
              <RecentMatchRow key={`${match.matchId}:${match.championId}`} match={match} champions={champions} />
            ))}
          </div>
        ) : (
          <p className="profile-empty-text">Recent matches will appear once stored games are attached to this alias.</p>
        )}
      </section>
    </div>
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
        <span>{formatNumber(record.games)} games · {record.winRate.toFixed(1)}% WR · {record.kda.toFixed(2)} KDA</span>
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
        <span><RoleIcon role={match.role} /> {roleLabel(match.role)} · {match.kills}/{match.deaths}/{match.assists} · Patch {match.patch}</span>
      </div>
    </div>
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
