import { useEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import { CalendarDays, CircleAlert, History, LoaderCircle, Package, RadioTower, Shield, Trophy } from 'lucide-react';
import { getLiveGame, getSummonerProfile, resolveAccountAlias } from '../api/client';
import type { AccountAliasMatch, ChampionData, ChampionRecord, ItemData, RankedRecord, RuneData, SummonerBuildRecord, SummonerProfile, SummonerRecentMatch, SummonerSpellData } from '../api/types';
import { platformLabel } from '../lib/lookup';
import {
  championByKey,
  championImageUrl,
  itemImageUrl,
  itemName,
  parseRuneSignature,
  profileIconUrl,
  rankIconUrl,
  rankLabel,
  runeImageUrl,
  runeName,
  runeStyleImageUrl,
  runeStyleName,
  signatureItems,
  signatureSpells,
  summonerSpellImageUrl,
  summonerSpellName,
} from '../lib/staticData';
import { LiveMatchups } from './LiveMatchups';
import { RoleIcon, roleLabel } from '../lib/roles';
import { MetricTile, MiniStat } from './ui/MetricTile';
import { EmptyState } from './ui/Panel';
import { RoleTabs } from './ui/RoleTabs';
import { SegmentedControl } from './ui/SegmentedControl';

type ProfileSection = 'overview' | 'champions' | 'builds' | 'matches';
type ChampionSort = 'games' | 'winrate' | 'kda';
type BuildSort = 'games' | 'winrate' | 'champion';
type MatchResultFilter = 'all' | 'wins' | 'losses';
type MatchRoleFilter = 'ALL' | 'TOP' | 'JUNGLE' | 'MIDDLE' | 'BOTTOM' | 'UTILITY';

const profileSections: { key: ProfileSection; label: string }[] = [
  { key: 'overview', label: 'Overview' },
  { key: 'champions', label: 'Champion Stats' },
  { key: 'builds', label: 'Builds' },
  { key: 'matches', label: 'Match History' },
];

const profileRoleFilters: MatchRoleFilter[] = ['ALL', 'TOP', 'JUNGLE', 'MIDDLE', 'BOTTOM', 'UTILITY'];
const profileRoleFilterOptions = profileRoleFilters.map((role) => ({ value: role, label: roleLabel(role) }));
const championSortOptions: { value: ChampionSort; label: string }[] = [
  { value: 'games', label: 'Games' },
  { value: 'winrate', label: 'Winrate' },
  { value: 'kda', label: 'KDA' },
];
const buildSortOptions: { value: BuildSort; label: string }[] = [
  { value: 'games', label: 'Games' },
  { value: 'winrate', label: 'Winrate' },
  { value: 'champion', label: 'Champion' },
];
const matchResultOptions: { value: MatchResultFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'wins', label: 'Wins' },
  { value: 'losses', label: 'Losses' },
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
  onBackgroundChampionScopeChange?: (championIds: number[]) => void;
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
  onBackgroundChampionScopeChange,
}: Props) {
  const [section, setSection] = useState<ProfileSection>('overview');
  const [liveViewDismissed, setLiveViewDismissed] = useState(false);
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
    enabled: Boolean(resolvedGameName && resolvedTagLine && !liveQuery.isFetching),
    retry: false,
    staleTime: 60_000,
  });

  useEffect(() => {
    setSection('overview');
    setLiveViewDismissed(false);
  }, [resolvedPlatform, resolvedGameName, resolvedTagLine]);

  useEffect(() => {
    if (foundAlias) {
      onResolvedAlias?.(foundAlias);
    }
  }, [foundAlias, onResolvedAlias]);

  const profileBackgroundChampionIds = useMemo(() => (
    profileQuery.data ? profileBackgroundChampionPool(profileQuery.data) : []
  ), [profileQuery.data]);

  useEffect(() => {
    onBackgroundChampionScopeChange?.([]);
  }, [onBackgroundChampionScopeChange, resolvedPlatform, resolvedGameName, resolvedTagLine]);

  useEffect(() => {
    if (profileBackgroundChampionIds.length) {
      onBackgroundChampionScopeChange?.(profileBackgroundChampionIds);
    }
  }, [onBackgroundChampionScopeChange, profileBackgroundChampionIds]);

  if (liveQuery.data && !liveViewDismissed) {
    return (
      <section className="profile-live-shell live-panel has-game">
        <LiveMatchups
          liveGame={liveQuery.data}
          champions={champions}
          items={items}
          profileAction={{
            label: 'View Profile',
            onClick: () => setLiveViewDismissed(true),
          }}
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
  const summonerIcon = profileIconUrl(champions?.version, profileQuery.data?.summoner?.profileIconId);
  const summonerLevel = profileQuery.data?.summoner?.summonerLevel;

  return (
    <section className="profile-page">
      <div className="profile-card">
        <div className="profile-card-header">
          {summonerIcon ? (
            <img className="profile-card-icon" src={summonerIcon} alt={`${exactRiotId} profile icon`} />
          ) : (
            <RadioTower size={24} />
          )}
          <div>
            <span>Summoner Profile</span>
            <strong>{exactRiotId || 'Search for a Riot ID'}</strong>
          </div>
          <em>{platformLabel(resolvedPlatform)}{summonerLevel ? ` · Level ${formatNumber(summonerLevel)}` : ''}</em>
        </div>

        {resolvedGameName && resolvedTagLine ? (
          <div className="profile-action-row">
            <button
              className={`profile-action-button ${liveQuery.data ? 'live' : ''}`}
              disabled={!liveQuery.data}
              onClick={() => setLiveViewDismissed(false)}
              type="button"
            >
              <RadioTower size={16} />
              <span>Live Match</span>
              <em>{liveQuery.data ? 'Live now' : liveQuery.isLoading ? 'Checking...' : 'Not live'}</em>
            </button>
          </div>
        ) : null}

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
          <StoredProfile
            profile={profileQuery.data}
            champions={champions}
            items={items}
            spells={spells}
            runes={runes}
            section={section}
            onSectionChange={setSection}
          />
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

function StoredProfile({
  profile,
  champions,
  items,
  spells,
  runes,
  section,
  onSectionChange,
}: {
  profile: SummonerProfile;
  champions?: ChampionData;
  items?: ItemData;
  spells?: SummonerSpellData;
  runes?: RuneData;
  section: ProfileSection;
  onSectionChange: (section: ProfileSection) => void;
}) {
  const rank = profile.rank;
  const summary = profile.summary;
  const bestChampion = profile.topChampions[0];
  const freshness = profileFreshness(summary);
  return (
    <div className="profile-data-stack">
      <div className="profile-snapshot-strip">
        <ProfileSnapshot icon={<Shield size={16} />} label="Solo/Duo" value={rank ? rankLabel(rank) : 'Rank Unknown'} detail={rank ? `${rank.leaguePoints} LP · ${rank.winRate.toFixed(1)}% WR` : 'No cached rank snapshot'} />
        <ProfileSnapshot icon={<Trophy size={16} />} label="Stored Form" value={`${summary.winRate.toFixed(1)}% WR`} detail={`${formatNumber(summary.games)} games · ${summary.kda ? summary.kda.toFixed(2) : '--'} KDA`} />
        <ProfileSnapshot icon={<Trophy size={16} />} label="Main Champion" value={bestChampion ? championName(champions, bestChampion.championId) : 'No Sample'} detail={bestChampion ? `${formatNumber(bestChampion.games)} games · ${bestChampion.winRate.toFixed(1)}% WR` : 'Collector has not reached champion history'} />
        <ProfileSnapshot icon={<CalendarDays size={16} />} label="Last Stored" value={formatProfileDate(summary.lastSeen)} detail={freshness.snapshotDetail} />
      </div>

      <ProfileFreshnessBanner freshness={freshness} />

      <SegmentedControl
        ariaLabel="Summoner profile sections"
        className="profile-section-tabs"
        options={profileSections.map((candidate) => ({ value: candidate.key, label: candidate.label }))}
        value={section}
        onChange={onSectionChange}
      />

      {section === 'overview' ? (
        <OverviewTab profile={profile} champions={champions} />
      ) : null}

      {section === 'champions' ? (
        <ChampionPoolTab champions={champions} records={profile.topChampions} roleRecords={profile.topChampionRoles ?? []} />
      ) : null}

      {section === 'builds' ? (
        <BuildsUsedTab
          builds={profile.topBuilds ?? []}
          champions={champions}
          items={items}
          spells={spells}
          runes={runes}
        />
      ) : null}

      {section === 'matches' ? (
        <MatchHistoryTab champions={champions} matches={profile.recentMatches} />
      ) : null}
    </div>
  );
}

function OverviewTab({ profile, champions }: { profile: SummonerProfile; champions?: ChampionData }) {
  const rank = profile.rank;
  const summary = profile.summary;
  const bestChampion = profile.topChampions[0];
  const recent = profile.recentMatches.slice(0, 10);
  const recentWins = recent.filter((match) => match.win).length;
  const recentWinRate = recent.length ? (recentWins / recent.length) * 100 : 0;
  const recentChampion = recent[0] ? championName(champions, recent[0].championId) : 'No recent sample';
  return (
    <div className="profile-data-grid profile-overview-grid">
      <RankedPanel rank={rank} />
      <FormPanel summary={summary} />
      <DataWindowPanel summary={summary} />

      <section className="profile-panel profile-wide-panel profile-overview-read">
        <PanelHeading icon={<Trophy size={16} />} title="Profile Read" />
        <div className="profile-read-layout">
          <div className="profile-read-main">
            <span>Best Stored Champion</span>
            <strong>{bestChampion ? championName(champions, bestChampion.championId) : 'No Champion Sample'}</strong>
            <p>
              {bestChampion
                ? `${formatNumber(bestChampion.games)} games, ${bestChampion.winRate.toFixed(1)}% winrate, ${bestChampion.kda.toFixed(2)} KDA in WinRift's stored ranked data.`
                : 'The collector has not attached enough ranked games to this profile yet.'}
            </p>
          </div>
          <div className="profile-read-side">
            <MetricTile className="profile-metric" label="Recent WR" value={recent.length ? `${recentWinRate.toFixed(1)}%` : '--'} />
            <MetricTile className="profile-metric" label="Recent Games" value={formatNumber(recent.length)} />
            <MetricTile className="profile-metric" label="Latest Champ" value={recentChampion} />
          </div>
        </div>
        {recent.length ? (
          <div className="profile-recent-pips" aria-label="Recent stored match results">
            {recent.map((match) => (
              <span key={`${match.matchId}:${match.championId}`} className={match.win ? 'win' : 'loss'} title={`${match.win ? 'Win' : 'Loss'} as ${championName(champions, match.championId)}`} />
            ))}
          </div>
        ) : null}
      </section>

      <ChampionHighlights champions={champions} records={profile.topChampions.slice(0, 4)} />
    </div>
  );
}

function ChampionHighlights({ champions, records }: { champions?: ChampionData; records: ChampionRecord[] }) {
  return (
    <section className="profile-panel profile-wide-panel">
      <PanelHeading icon={<Trophy size={16} />} title="Champion Highlights" />
      {records.length ? (
        <div className="profile-highlight-grid">
          {records.map((record) => {
            const champion = championByKey(champions, record.championId);
            return (
              <div className="profile-highlight-card" key={record.championId}>
                <img src={championImageUrl(champions, record.championId)} alt={champion?.name ?? String(record.championId)} />
                <div>
                  <strong>{champion?.name ?? `Champion ${record.championId}`}</strong>
                  <span>{formatNumber(record.games)} games</span>
                </div>
                <b>{record.winRate.toFixed(1)}%</b>
              </div>
            );
          })}
        </div>
      ) : (
        <EmptyState className="profile-empty-state" icon={false} title="No champion highlights yet" body="Champion highlights will appear once stored games are attached to this alias." />
      )}
    </section>
  );
}

function ChampionPoolTab({ champions, records, roleRecords }: { champions?: ChampionData; records: ChampionRecord[]; roleRecords: ChampionRecord[] }) {
  const [sort, setSort] = useState<ChampionSort>('games');
  const [roleFilter, setRoleFilter] = useState<MatchRoleFilter>('ALL');
  const [query, setQuery] = useState('');
  const activeRecords = useMemo(() => (
    roleFilter === 'ALL' ? records : roleRecords.filter((record) => record.role === roleFilter)
  ), [records, roleFilter, roleRecords]);
  const filteredRecords = useMemo(() => filterChampionRecords(activeRecords, query, champions), [activeRecords, champions, query]);
  const sortedRecords = useMemo(() => sortChampionRecords(filteredRecords, sort, champions), [champions, filteredRecords, sort]);
  const bestWinrate = sortedByWinRate(filteredRecords)[0];
  return (
    <section className="profile-panel profile-wide-panel profile-tab-panel">
      <div className="profile-tab-header">
        <PanelHeading icon={<Trophy size={16} />} title="Champion Pool" />
        <div className="profile-tab-controls">
          <label className="profile-tab-search">
            <span>Filter</span>
            <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Champion name" />
          </label>
          <SegmentedControl ariaLabel="Champion pool sorting" options={championSortOptions} value={sort} onChange={setSort} />
        </div>
      </div>
      <RoleTabs
        ariaLabel="Champion comfort role filters"
        className="profile-role-filter"
        options={profileRoleFilterOptions}
        value={roleFilter}
        onChange={(role) => setRoleFilter(role as MatchRoleFilter)}
      />
      <div className="profile-tab-summary">
        <MetricTile className="profile-metric" label="Shown Champions" value={`${formatNumber(filteredRecords.length)} / ${formatNumber(activeRecords.length)}`} />
        <MetricTile className="profile-metric" label="Tracked Games" value={formatNumber(filteredRecords.reduce((sum, record) => sum + record.games, 0))} />
        <MetricTile className="profile-metric" label="Best WR" value={bestWinrate ? `${championName(champions, bestWinrate.championId)} ${bestWinrate.winRate.toFixed(1)}%` : '--'} />
      </div>
      {sortedRecords.length ? (
        <div className="profile-champion-list">
          {sortedRecords.map((record) => (
            <ChampionComfortRow key={`${record.championId}:${record.role ?? 'ALL'}`} record={record} champions={champions} />
          ))}
        </div>
      ) : (
        <EmptyState className="profile-empty-state" icon={false} title={championPoolEmptyTitle(records, activeRecords, roleFilter)} body={championPoolEmptyBody(records, activeRecords, roleFilter)} />
      )}
    </section>
  );
}

function BuildsUsedTab({
  builds,
  champions,
  items,
  spells,
  runes,
}: {
  builds: SummonerBuildRecord[];
  champions?: ChampionData;
  items?: ItemData;
  spells?: SummonerSpellData;
  runes?: RuneData;
}) {
  const [sort, setSort] = useState<BuildSort>('games');
  const [roleFilter, setRoleFilter] = useState<MatchRoleFilter>('ALL');
  const filteredBuilds = useMemo(() => builds.filter((build) => roleFilter === 'ALL' || build.role === roleFilter), [builds, roleFilter]);
  const sortedBuilds = useMemo(() => sortBuildRecords(filteredBuilds, sort, champions), [champions, filteredBuilds, sort]);
  const totalGames = filteredBuilds.reduce((sum, build) => sum + build.games, 0);
  const wins = filteredBuilds.reduce((sum, build) => sum + build.wins, 0);
  const uniqueChampions = new Set(filteredBuilds.map((build) => build.championId)).size;
  return (
    <section className="profile-panel profile-wide-panel profile-tab-panel profile-builds-tab">
      <div className="profile-tab-header">
        <PanelHeading icon={<Package size={16} />} title="Builds" />
        <SegmentedControl ariaLabel="Summoner build sorting" options={buildSortOptions} value={sort} onChange={setSort} />
      </div>
      <RoleTabs
        ariaLabel="Summoner build role filters"
        className="profile-role-filter"
        options={profileRoleFilterOptions}
        value={roleFilter}
        onChange={(role) => setRoleFilter(role as MatchRoleFilter)}
      />
      <div className="profile-tab-summary">
        <MetricTile className="profile-metric" label="Build Paths" value={formatNumber(filteredBuilds.length)} />
        <MetricTile className="profile-metric" label="Tracked Games" value={formatNumber(totalGames)} />
        <MetricTile className="profile-metric" label="Winrate" value={totalGames ? `${((wins / totalGames) * 100).toFixed(1)}%` : '--'} />
        <MetricTile className="profile-metric" label="Champions" value={formatNumber(uniqueChampions)} />
      </div>
      <p className="profile-context-note">
        This is usage history from stored ranked games for this summoner, not generalized build advice.
      </p>
      {sortedBuilds.length ? (
        <div className="profile-build-list">
          {sortedBuilds.map((build) => (
            <BuildUsageRow
              key={`${build.championId}:${build.role}:${build.finalItemsSignature}:${build.core2Signature}:${build.core3Signature}:${build.runeSignature}:${build.spellSignature}`}
              build={build}
              champions={champions}
              items={items}
              spells={spells}
              runes={runes}
            />
          ))}
        </div>
      ) : (
        <EmptyState
          className="profile-empty-state"
          icon={false}
          title={builds.length ? 'No build paths match these filters' : 'No build paths yet'}
          body={builds.length ? 'Try a different role filter or switch back to all roles.' : 'Build usage appears after this summoner has stored ranked games with complete item paths.'}
        />
      )}
    </section>
  );
}

function MatchHistoryTab({ champions, matches }: { champions?: ChampionData; matches: SummonerRecentMatch[] }) {
  const [resultFilter, setResultFilter] = useState<MatchResultFilter>('all');
  const [roleFilter, setRoleFilter] = useState<MatchRoleFilter>('ALL');
  const filteredMatches = useMemo(() => matches.filter((match) => {
    const resultMatches = resultFilter === 'all' || (resultFilter === 'wins' ? match.win : !match.win);
    const roleMatches = roleFilter === 'ALL' || match.role === roleFilter;
    return resultMatches && roleMatches;
  }), [matches, resultFilter, roleFilter]);
  const wins = filteredMatches.filter((match) => match.win).length;
  const winRate = filteredMatches.length ? (wins / filteredMatches.length) * 100 : 0;
  return (
    <section className="profile-panel profile-wide-panel profile-tab-panel">
      <div className="profile-tab-header">
        <PanelHeading icon={<History size={16} />} title="Match History" />
        <SegmentedControl ariaLabel="Match result filters" options={matchResultOptions} value={resultFilter} onChange={setResultFilter} />
      </div>
      <RoleTabs
        ariaLabel="Match role filters"
        className="profile-role-filter"
        options={profileRoleFilterOptions}
        value={roleFilter}
        onChange={(role) => setRoleFilter(role as MatchRoleFilter)}
      />
      <div className="profile-tab-summary">
        <MetricTile className="profile-metric" label="Shown Games" value={formatNumber(filteredMatches.length)} />
        <MetricTile className="profile-metric" label="Winrate" value={filteredMatches.length ? `${winRate.toFixed(1)}%` : '--'} />
        <MetricTile className="profile-metric" label="Window" value={matches.length ? `${formatGameDate(matches[matches.length - 1].gameStartTimestamp)} to ${formatGameDate(matches[0].gameStartTimestamp)}` : '--'} />
      </div>
      {filteredMatches.length ? (
        <div className="profile-match-list">
          {filteredMatches.map((match) => (
            <RecentMatchRow key={`${match.matchId}:${match.championId}`} match={match} champions={champions} />
          ))}
        </div>
      ) : (
        <EmptyState
          className="profile-empty-state"
          icon={false}
          title={matches.length ? 'No matches match these filters' : 'No recent stored matches yet'}
          body={matches.length ? 'Relax the result or role filter to bring games back into view.' : 'Recent match cards appear once the collector reaches this player in ranked Solo/Duo data.'}
        />
      )}
    </section>
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

type ProfileFreshness = {
  tone: 'fresh' | 'recent' | 'stale' | 'empty';
  label: string;
  body: string;
  snapshotDetail: string;
};

function ProfileFreshnessBanner({ freshness }: { freshness: ProfileFreshness }) {
  return (
    <div className={`profile-freshness-banner ${freshness.tone}`}>
      <span>{freshness.label}</span>
      <p>{freshness.body}</p>
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
          <MetricTile className="profile-metric" label="Winrate" value={`${rank.winRate.toFixed(1)}%`} />
          <MetricTile className="profile-metric" label="Wins" value={formatNumber(rank.wins)} />
          <MetricTile className="profile-metric" label="Losses" value={formatNumber(rank.losses)} />
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
        <MetricTile className="profile-metric" label="Games" value={formatNumber(summary.games)} />
        <MetricTile className="profile-metric" label="Winrate" value={`${summary.winRate.toFixed(1)}%`} />
        <MetricTile className="profile-metric" label="KDA" value={summary.kda ? summary.kda.toFixed(2) : '--'} />
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
        <MetricTile className="profile-metric" label="First Seen" value={formatProfileDate(summary.firstSeen)} />
        <MetricTile className="profile-metric" label="Last Seen" value={formatProfileDate(summary.lastSeen)} />
        <MetricTile className="profile-metric" label="Sample" value={`${formatNumber(summary.games)} games`} />
      </div>
      <div className="profile-kda-line">
        Profile summaries are compact read-model rows refreshed from retained ranked Solo/Duo matches.
      </div>
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

function ChampionComfortRow({ record, champions }: { record: ChampionRecord; champions?: ChampionData }) {
  const champion = championByKey(champions, record.championId);
  return (
    <div className="profile-champion-row">
      <img src={championImageUrl(champions, record.championId)} alt={champion?.name ?? String(record.championId)} />
      <div>
        <strong>{champion?.name ?? `Champion ${record.championId}`}</strong>
        <span>{record.role ? <><RoleIcon role={record.role} /> {roleLabel(record.role)} · </> : null}{record.avgKills.toFixed(1)} / {record.avgDeaths.toFixed(1)} / {record.avgAssists.toFixed(1)} average</span>
      </div>
      <div className="profile-row-stats">
        <MiniStat label="WR" value={`${record.winRate.toFixed(1)}%`} />
        <MiniStat label="KDA" value={record.kda.toFixed(2)} />
        <MiniStat label="Games" value={formatNumber(record.games)} />
      </div>
    </div>
  );
}

function RecentMatchRow({ match, champions }: { match: SummonerRecentMatch; champions?: ChampionData }) {
  const champion = championByKey(champions, match.championId);
  const championLabel = champion?.name ?? `Champion ${match.championId}`;
  return (
    <div className={`profile-match-row ${match.win ? 'win' : 'loss'}`}>
      <img src={championImageUrl(champions, match.championId)} alt={champion?.name ?? String(match.championId)} />
      <span className={`profile-match-result-badge ${match.win ? 'win' : 'loss'}`}>{match.win ? 'Win' : 'Loss'}</span>
      <div>
        <strong>{championLabel}</strong>
        <span className="profile-match-meta"><RoleIcon role={match.role} /> {roleLabel(match.role)} · Patch {match.patch} · {formatGameDate(match.gameStartTimestamp)} · {formatDuration(match.durationSeconds)}</span>
      </div>
      <div className="profile-row-stats">
        <MiniStat label="KDA" value={`${match.kills}/${match.deaths}/${match.assists}`} />
        <MiniStat label="Role" value={roleLabel(match.role)} />
        <MiniStat label="Duration" value={formatDuration(match.durationSeconds)} />
      </div>
    </div>
  );
}

function BuildUsageRow({
  build,
  champions,
  items,
  spells,
  runes,
}: {
  build: SummonerBuildRecord;
  champions?: ChampionData;
  items?: ItemData;
  spells?: SummonerSpellData;
  runes?: RuneData;
}) {
  const champion = championByKey(champions, build.championId);
  const coreItems = signatureItems(build.core3Signature || build.core2Signature);
  const finalItems = signatureItems(build.finalItemsSignature);
  const displayedCore = coreItems.length ? coreItems : finalItems.slice(0, 3);
  const parsedRunes = parseRuneSignature(build.runeSignature);
  const primaryRuneStyleSrc = runeStyleImageUrl(runes, parsedRunes.primaryStyleId);
  const runeIds = parsedRunes.runeIds.slice(0, 4);
  const spellIds = signatureSpells(build.spellSignature);
  return (
    <div className="profile-build-row">
      <div className="profile-build-identity">
        <img src={championImageUrl(champions, build.championId)} alt={champion?.name ?? String(build.championId)} />
        <div>
          <strong>{champion?.name ?? `Champion ${build.championId}`}</strong>
          <span><RoleIcon role={build.role} /> {roleLabel(build.role)}</span>
        </div>
      </div>
      <div className="profile-build-paths">
        <div className="profile-build-path">
          <em>Core</em>
          <ItemIconList itemIds={displayedCore} items={items} />
        </div>
        <div className="profile-build-path">
          <em>Final</em>
          <ItemIconList itemIds={finalItems} items={items} />
        </div>
      </div>
      <div className="profile-build-loadout">
        <div>
          <em>Runes</em>
          <div className="profile-build-icon-row">
            {primaryRuneStyleSrc ? (
              <img src={primaryRuneStyleSrc} alt={runeStyleName(runes, parsedRunes.primaryStyleId)} title={runeStyleName(runes, parsedRunes.primaryStyleId)} />
            ) : null}
            {runeIds.map((runeId) => {
              const src = runeImageUrl(runes, runeId);
              return src ? <img key={runeId} src={src} alt={runeName(runes, runeId)} title={runeName(runes, runeId)} /> : null;
            })}
          </div>
        </div>
        <div>
          <em>Spells</em>
          <div className="profile-build-icon-row">
            {spellIds.map((spellId) => {
              const src = summonerSpellImageUrl(spells, spellId);
              return src ? <img key={spellId} src={src} alt={summonerSpellName(spells, spellId)} title={summonerSpellName(spells, spellId)} /> : null;
            })}
          </div>
        </div>
      </div>
      <div className="profile-row-stats">
        <MiniStat label="Games" value={formatNumber(build.games)} />
        <MiniStat label="WR" value={`${build.winRate.toFixed(1)}%`} />
        <MiniStat label="KDA" value={build.kda.toFixed(2)} />
      </div>
    </div>
  );
}

function ItemIconList({ itemIds, items }: { itemIds: string[]; items?: ItemData }) {
  if (!itemIds.length) {
    return <span className="profile-build-empty-path">No item path</span>;
  }
  return (
    <div className="profile-build-icon-row">
      {itemIds.slice(0, 6).map((itemId, index) => {
        const src = itemImageUrl(items, itemId);
        return src ? (
          <img key={`${itemId}:${index}`} src={src} alt={itemName(items, itemId)} title={itemName(items, itemId)} />
        ) : (
          <span key={`${itemId}:${index}`} className="profile-build-item-fallback">{itemId}</span>
        );
      })}
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

function championName(champions: ChampionData | undefined, championId: number) {
  return championByKey(champions, championId)?.name ?? `Champion ${championId}`;
}

function filterChampionRecords(records: ChampionRecord[], query: string, champions: ChampionData | undefined) {
  const normalizedQuery = query.trim().toLowerCase();
  if (!normalizedQuery) {
    return records;
  }
  return records.filter((record) => {
    const name = championName(champions, record.championId).toLowerCase();
    return name.includes(normalizedQuery) || String(record.championId).includes(normalizedQuery);
  });
}

function championPoolEmptyTitle(overallRecords: ChampionRecord[], activeRecords: ChampionRecord[], roleFilter: MatchRoleFilter) {
  if (!overallRecords.length) {
    return 'No champion sample yet';
  }
  if (roleFilter !== 'ALL' && !activeRecords.length) {
    return `No ${roleLabel(roleFilter).toLowerCase()} champion sample yet`;
  }
  return 'No champions match that filter';
}

function championPoolEmptyBody(overallRecords: ChampionRecord[], activeRecords: ChampionRecord[], roleFilter: MatchRoleFilter) {
  if (!overallRecords.length) {
    return 'The collector has not attached stored ranked champion games to this alias yet.';
  }
  if (roleFilter !== 'ALL' && !activeRecords.length) {
    return 'Role-specific rows appear after the summoner champion role summary refreshes or the collector finds games in this role.';
  }
  return 'Clear the filter or try another champion name.';
}

function sortChampionRecords(records: ChampionRecord[], sort: ChampionSort, champions: ChampionData | undefined) {
  return [...records].sort((a, b) => {
    if (sort === 'winrate') {
      return b.winRate - a.winRate || b.games - a.games || championName(champions, a.championId).localeCompare(championName(champions, b.championId));
    }
    if (sort === 'kda') {
      return b.kda - a.kda || b.games - a.games || championName(champions, a.championId).localeCompare(championName(champions, b.championId));
    }
    return b.games - a.games || b.winRate - a.winRate || championName(champions, a.championId).localeCompare(championName(champions, b.championId));
  });
}

function sortBuildRecords(records: SummonerBuildRecord[], sort: BuildSort, champions: ChampionData | undefined) {
  return [...records].sort((a, b) => {
    if (sort === 'winrate') {
      return b.winRate - a.winRate || b.games - a.games || championName(champions, a.championId).localeCompare(championName(champions, b.championId));
    }
    if (sort === 'champion') {
      return championName(champions, a.championId).localeCompare(championName(champions, b.championId)) || b.games - a.games;
    }
    return b.games - a.games || b.winRate - a.winRate || championName(champions, a.championId).localeCompare(championName(champions, b.championId));
  });
}

function sortedByWinRate(records: ChampionRecord[]) {
  return [...records].sort((a, b) => b.winRate - a.winRate || b.games - a.games);
}

function formatProfileDate(value?: string) {
  if (!value) return '--';
  const date = new Date(value);
  if (Number.isNaN(date.getTime()) || date.getFullYear() <= 1970) return '--';
  return new Intl.DateTimeFormat('en-US', { month: 'short', day: 'numeric' }).format(date);
}

function profileFreshness(summary: SummonerProfile['summary']): ProfileFreshness {
  const days = daysSinceDate(summary.lastSeen);
  const firstSeen = formatProfileDate(summary.firstSeen);
  const lastSeen = formatProfileDate(summary.lastSeen);
  if (days === undefined) {
    return {
      tone: 'empty',
      label: 'No stored window yet',
      body: 'The collector has not attached retained ranked games to this profile yet. Live lookup can still work if the player is in game.',
      snapshotDetail: 'No retained games yet',
    };
  }
  const relativeLastSeen = relativeDayLabel(days);
  const sampleText = `${formatNumber(summary.games)} stored ${summary.games === 1 ? 'game' : 'games'}`;
  const firstSeenDetail = firstSeen !== '--' ? ` · first ${firstSeen}` : '';
  if (days <= 2) {
    return {
      tone: 'fresh',
      label: 'Fresh stored sample',
      body: `Last stored game was ${relativeLastSeen}. This profile is using ${sampleText}${firstSeen !== '--' ? ` since ${firstSeen}` : ''}.`,
      snapshotDetail: `${relativeLastSeen}${firstSeenDetail}`,
    };
  }
  if (days <= 14) {
    return {
      tone: 'recent',
      label: 'Recent stored sample',
      body: `Last stored game was ${relativeLastSeen}. Treat form and champion comfort as recent stored history, not live-season truth.`,
      snapshotDetail: `${relativeLastSeen}${firstSeenDetail}`,
    };
  }
  return {
    tone: 'stale',
    label: 'Aging stored sample',
    body: `Last stored game was ${relativeLastSeen} on ${lastSeen}. This profile may lag behind the player's current form until the collector sees newer games.`,
    snapshotDetail: `${relativeLastSeen}${firstSeenDetail}`,
  };
}

function profileBackgroundChampionPool(profile: SummonerProfile) {
  const championIds: number[] = [];
  const addChampion = (championId: number) => {
    if (championId && !championIds.includes(championId)) {
      championIds.push(championId);
    }
  };
  profile.recentMatches.slice(0, 10).forEach((match) => addChampion(match.championId));
  profile.topChampions.slice(0, 10).forEach((record) => addChampion(record.championId));
  return championIds.slice(0, 10);
}

function daysSinceDate(value?: string) {
  if (!value) return undefined;
  const date = new Date(value);
  if (Number.isNaN(date.getTime()) || date.getFullYear() <= 1970) {
    return undefined;
  }
  const now = Date.now();
  const diff = now - date.getTime();
  if (diff < 0) {
    return 0;
  }
  return Math.floor(diff / 86_400_000);
}

function relativeDayLabel(days: number) {
  if (days <= 0) return 'today';
  if (days === 1) return 'yesterday';
  return `${formatNumber(days)} days ago`;
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
