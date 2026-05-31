import { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { CircleAlert, LoaderCircle } from 'lucide-react';
import { getLiveGame, getSummonerProfile, resolveAccountAlias } from '../api/client';
import type { AccountAliasMatch, ChampionData, ItemData, RuneData, SummonerProfile, SummonerSpellData } from '../api/types';
import { platformLabel } from '../lib/lookup';
import { LiveMatchups } from './LiveMatchups';
import { ProfileHeader } from './summoner-profile/ProfileHeader';
import { ProfileMessage } from './summoner-profile/ProfileMessage';
import { StoredProfile, type ProfileSection } from './summoner-profile/ProfileSections';
import { SummonerHub } from './summoner-profile/SummonerHub';

type Props = {
  platform?: string;
  gameName?: string;
  tagLine?: string;
  champions?: ChampionData;
  items?: ItemData;
  analyticsPatch?: string;
  spells?: SummonerSpellData;
  runes?: RuneData;
  onUseAlias: (alias: AccountAliasMatch) => void;
  onSearch: (gameName: string, tagLine: string, platform: string) => void;
  onResolvedAlias?: (alias: AccountAliasMatch) => void;
  onBackgroundChampionScopeChange?: (championIds: number[]) => void;
};

export function SummonerProfilePage({
  platform = 'NA1',
  gameName,
  tagLine,
  champions,
  items,
  analyticsPatch,
  spells,
  runes,
  onUseAlias,
  onSearch,
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
          analyticsPatch={analyticsPatch}
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

  return (
    <section className="profile-page">
      <div className="profile-card">
        <ProfileHeader
          exactRiotId={exactRiotId}
          platform={resolvedPlatform}
          staticVersion={champions?.version}
          profileIconId={profileQuery.data?.summoner?.profileIconId}
          summonerLevel={profileQuery.data?.summoner?.summonerLevel}
          canCheckLive={Boolean(resolvedGameName && resolvedTagLine)}
          liveAvailable={Boolean(liveQuery.data)}
          liveLoading={liveQuery.isLoading}
          onShowLive={() => setLiveViewDismissed(false)}
        />

        {!gameName ? (
          <SummonerHub
            champions={champions}
            initialPlatform={resolvedPlatform}
            onSearch={onSearch}
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

function profileLiveError(riotId: string, message: string) {
  const normalized = message.toLowerCase();
  if (normalized.includes('not currently in a live game') || normalized.includes('riot id not found')) {
    return `Summoner '${riotId.trim()}' is not currently in a live match`;
  }
  return message;
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
