import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { CircleAlert, LoaderCircle, Search, UserRound } from 'lucide-react';
import { getSummonerLeaderboard, resolveAccountAlias, searchAccountAliases } from '../../api/client';
import type { AccountAliasMatch, ChampionData, SummonerLeaderboardRow } from '../../api/types';
import { parseRiotId, platformLabel, platforms } from '../../lib/lookup';
import { profileIconUrl, rankIconUrl, rankLabel } from '../../lib/staticData';
import { EmptyState } from '../ui/Panel';
import { ProfileMessage } from './ProfileMessage';

export function SummonerHub({
  champions,
  initialPlatform,
  onSearch,
}: {
  champions?: ChampionData;
  initialPlatform: string;
  onSearch: (gameName: string, tagLine: string, platform: string) => void;
}) {
  const [riotId, setRiotId] = useState('');
  const [platform, setPlatform] = useState(initialPlatform || 'NA1');
  const [showPlatforms, setShowPlatforms] = useState(false);
  const [validationError, setValidationError] = useState('');
  const [aliasLoading, setAliasLoading] = useState(false);
  const [aliasSuggestions, setAliasSuggestions] = useState<AccountAliasMatch[]>([]);
  const [suggestionsOpen, setSuggestionsOpen] = useState(false);
  const selectedPlatform = platforms.find((candidate) => candidate.value === platform) ?? platforms[0];
  const parsedInput = parseRiotId(riotId);
  const showAliasSuggestions = !showPlatforms && suggestionsOpen && !parsedInput.tagLine && aliasSuggestions.length > 0;
  const leaderboard = useQuery({
    queryKey: ['summoner-leaderboard', platform],
    queryFn: () => getSummonerLeaderboard(platform, 50),
    staleTime: 60_000,
  });

  useEffect(() => {
    setPlatform(initialPlatform || 'NA1');
  }, [initialPlatform]);

  useEffect(() => {
    const parsed = parseRiotId(riotId);
    if (parsed.tagLine || parsed.gameName.length < 2) {
      setAliasSuggestions([]);
      setSuggestionsOpen(false);
      return;
    }
    let cancelled = false;
    const timer = window.setTimeout(async () => {
      try {
        const response = await searchAccountAliases(parsed.gameName, platform, 7);
        if (cancelled) {
          return;
        }
        setAliasSuggestions(response.matches);
        setSuggestionsOpen(response.matches.length > 0);
      } catch {
        if (!cancelled) {
          setAliasSuggestions([]);
          setSuggestionsOpen(false);
        }
      }
    }, 180);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [riotId, platform]);

  const selectAlias = (alias: AccountAliasMatch) => {
    const resolvedRiotID = `${alias.gameName}#${alias.tagLine}`;
    setRiotId(resolvedRiotID);
    setValidationError('');
    setSuggestionsOpen(false);
    onSearch(alias.gameName, alias.tagLine, alias.platform || platform);
  };

  const search = async () => {
    const parsed = parseRiotId(riotId);
    if (!parsed.gameName) {
      setValidationError('Enter a Riot ID to search.');
      return;
    }
    if (!parsed.tagLine) {
      setValidationError('');
      setAliasLoading(true);
      try {
        const alias = await resolveAccountAlias(parsed.gameName, platform);
        if (alias.status === 'found' && alias.gameName && alias.tagLine) {
          const resolvedRiotID = `${alias.gameName}#${alias.tagLine}`;
          setRiotId(resolvedRiotID);
          setSuggestionsOpen(false);
          onSearch(alias.gameName, alias.tagLine, alias.platform || platform);
          return;
        }
        if (alias.status === 'ambiguous') {
          setValidationError(`Tag required. Multiple saved Riot IDs match '${parsed.gameName}'. Use Name#Tag.`);
          return;
        }
        setValidationError(`Tag required. No unique saved Riot ID for '${parsed.gameName}'. Use Name#Tag.`);
      } catch {
        setValidationError('Tag required. Saved Riot ID lookup failed; use Name#Tag.');
      } finally {
        setAliasLoading(false);
      }
      return;
    }
    setValidationError('');
    setSuggestionsOpen(false);
    onSearch(parsed.gameName, parsed.tagLine, platform);
  };

  return (
    <div className="summoner-hub">
      <div className="summoner-hub-intro">
        <span>Summoner Lookup</span>
        <strong>Profiles and Stored Ranked Ladder</strong>
        <p>Search a Riot ID to open a profile. If that player is in a live game, WinRift jumps straight into the match room.</p>
      </div>
      <div className="summoner-search-shell">
        <div className={validationError ? 'search-bar invalid summoner-hub-search' : 'search-bar summoner-hub-search'}>
          <UserRound className="search-mark" size={22} />
          <input
            value={riotId}
            placeholder="Riot ID, e.g. Sneaky#NA69"
            onChange={(event) => {
              setRiotId(event.target.value);
              setValidationError('');
            }}
            onFocus={() => setSuggestionsOpen(aliasSuggestions.length > 0)}
            onKeyDown={(event) => event.key === 'Enter' && search()}
          />
          <button
            className="server-button"
            onClick={() => setShowPlatforms((visible) => !visible)}
            style={{ backgroundColor: selectedPlatform.color }}
            type="button"
          >
            {selectedPlatform.label}
          </button>
          <button className="search-button" onClick={search} title="Search summoners" aria-label="Search summoners" type="button">
            <Search size={19} />
            <span>Search</span>
          </button>
        </div>
        {showPlatforms ? (
          <div className="server-options-row summoner-server-options">
            {platforms.map((candidate) => (
              <button
                key={candidate.value}
                className={candidate.value === platform ? 'server-option selected' : 'server-option'}
                onClick={() => {
                  setPlatform(candidate.value);
                  setShowPlatforms(false);
                }}
                style={{ backgroundColor: candidate.color }}
                type="button"
              >
                {candidate.label}
              </button>
            ))}
          </div>
        ) : null}
        {showAliasSuggestions ? (
          <div className="search-autocomplete summoner-search-autocomplete">
            {aliasSuggestions.map((alias) => (
              <button
                key={`${alias.platform}:${alias.puuid}:${alias.tagLine}`}
                className="alias-option"
                onClick={() => selectAlias(alias)}
                aria-label={`Use ${alias.gameName}#${alias.tagLine}`}
                type="button"
              >
                <span className="alias-name">{alias.gameName}</span>
                <span className="alias-tag">#{alias.tagLine}</span>
                <span className="alias-platform">{platformLabel(alias.platform)}</span>
              </button>
            ))}
          </div>
        ) : null}
      </div>
      <div className={aliasLoading || validationError ? 'lookup-status has-message' : 'lookup-status'} aria-live="polite">
        {aliasLoading ? (
          <div className="search-message-card checking">
            <LoaderCircle size={16} aria-hidden="true" />
            <span>Checking saved Riot IDs...</span>
          </div>
        ) : null}
        {validationError ? (
          <div className="search-message-card error">
            <CircleAlert size={16} aria-hidden="true" />
            <span>{validationError}</span>
          </div>
        ) : null}
      </div>

      <SummonerLeaderboard
        champions={champions}
        loading={leaderboard.isLoading}
        error={leaderboard.error instanceof Error ? leaderboard.error : undefined}
        rows={leaderboard.data?.results ?? []}
        platform={platform}
        onSelect={onSearch}
      />
    </div>
  );
}

function SummonerLeaderboard({
  champions,
  loading,
  error,
  rows,
  platform,
  onSelect,
}: {
  champions?: ChampionData;
  loading: boolean;
  error?: Error;
  rows: SummonerLeaderboardRow[];
  platform: string;
  onSelect: (gameName: string, tagLine: string, platform: string) => void;
}) {
  return (
    <section className="summoner-leaderboard-panel">
      <div className="summoner-leaderboard-header">
        <div>
          <span>Stored Ranked Ladder</span>
          <strong>{platformLabel(platform)} Solo/Duo</strong>
        </div>
        <em>Cached rank snapshots from summoners WinRift has seen</em>
      </div>
      {loading ? (
        <ProfileMessage
          icon={<LoaderCircle size={17} className="spin-icon" />}
          title="Loading ladder"
          body={`Checking cached ranked snapshots for ${platformLabel(platform)}.`}
        />
      ) : null}
      {error ? (
        <ProfileMessage
          icon={<CircleAlert size={17} />}
          title="Leaderboard unavailable"
          body={error.message}
        />
      ) : null}
      {!loading && !error && rows.length ? (
        <div className="summoner-leaderboard-list">
          {rows.map((row) => (
            <SummonerLeaderboardRowButton
              key={`${row.platform}:${row.puuid}:${row.gameName}:${row.tagLine}`}
              champions={champions}
              row={row}
              onSelect={onSelect}
            />
          ))}
        </div>
      ) : null}
      {!loading && !error && !rows.length ? (
        <EmptyState
          className="profile-empty-state"
          icon={false}
          title="No cached ranked ladder yet"
          body="The rank worker has not cached enough named summoners for this region yet."
        />
      ) : null}
    </section>
  );
}

function SummonerLeaderboardRowButton({
  champions,
  row,
  onSelect,
}: {
  champions?: ChampionData;
  row: SummonerLeaderboardRow;
  onSelect: (gameName: string, tagLine: string, platform: string) => void;
}) {
  const totalGames = row.ranked.totalGames || row.rankedGames;
  const storedDetail = row.storedGames > 0 ? `${formatNumber(row.storedGames)} stored · ${row.storedWinRate.toFixed(1)}% WR` : 'No stored profile sample yet';
  const iconSrc = profileIconUrl(champions?.version, row.profileIconId);
  return (
    <button
      className="summoner-leaderboard-row"
      onClick={() => onSelect(row.gameName, row.tagLine, row.platform)}
      type="button"
    >
      <span className="summoner-ladder-rank">#{row.rank}</span>
      {iconSrc ? (
        <img className="summoner-ladder-profile-icon" src={iconSrc} alt={`${row.gameName} profile icon`} />
      ) : (
        <span className="summoner-ladder-icon-fallback"><UserRound size={17} /></span>
      )}
      <span className="summoner-ladder-name">
        <strong>{row.gameName}</strong>
        <em>#{row.tagLine} · {platformLabel(row.platform)}</em>
      </span>
      <span className="summoner-ladder-tier">
        <img className="summoner-ladder-rank-icon" src={rankIconUrl(row.ranked)} alt={rankLabel(row.ranked)} />
        <span>
          <strong>{rankLabel(row.ranked)}</strong>
          <em>{row.ranked.leaguePoints} LP</em>
        </span>
      </span>
      <span className="summoner-ladder-record">
        <strong>{row.ranked.winRate.toFixed(1)}%</strong>
        <em>{formatNumber(totalGames)} ranked games</em>
      </span>
      <span className="summoner-ladder-stored">{storedDetail}</span>
    </button>
  );
}

function formatNumber(value: number) {
  return new Intl.NumberFormat('en-US').format(value);
}
