import { CircleAlert, LoaderCircle, RadioTower, Search } from 'lucide-react';
import { useEffect, useState } from 'react';
import { resolveAccountAlias, searchAccountAliases } from '../api/client';
import type { AccountAliasMatch, ChampionData, ItemData, LiveGame, RuneData, SummonerSpellData } from '../api/types';
import { LiveMatchups } from './LiveMatchups';

const platforms = [
  { value: 'NA1', label: 'NA', color: 'red' },
  { value: 'EUW1', label: 'EUW', color: 'blue' },
  { value: 'EUN1', label: 'EUNE', color: 'darkgreen' },
  { value: 'LA1', label: 'LAN', color: 'darkorange' },
  { value: 'LA2', label: 'LAS', color: 'firebrick' },
  { value: 'BR1', label: 'BR', color: 'forestgreen' },
  { value: 'TR1', label: 'TR', color: 'indigo' },
  { value: 'RU', label: 'RU', color: 'darkred' },
  { value: 'KR', label: 'KR', color: 'navy' },
  { value: 'JP1', label: 'JP', color: 'crimson' },
  { value: 'OC1', label: 'OCE', color: 'steelblue' },
] as const;

type Props = {
  champions?: ChampionData;
  items?: ItemData;
  spells?: SummonerSpellData;
  runes?: RuneData;
  liveGame?: LiveGame;
  loading: boolean;
  error?: Error | null;
  onSearch: (gameName: string, tagLine: string, platform: string) => void;
};

export function LiveMatchPanel({ champions, items, spells, runes, liveGame, loading, error, onSearch }: Props) {
  const [riotId, setRiotId] = useState('');
  const [platform, setPlatform] = useState('NA1');
  const [showPlatforms, setShowPlatforms] = useState(false);
  const [lastSearch, setLastSearch] = useState('');
  const [validationError, setValidationError] = useState('');
  const [aliasLoading, setAliasLoading] = useState(false);
  const [aliasSuggestions, setAliasSuggestions] = useState<AccountAliasMatch[]>([]);
  const [suggestionsOpen, setSuggestionsOpen] = useState(false);
  const selectedPlatform = platforms.find((candidate) => candidate.value === platform) ?? platforms[0];
  const parsedInput = parseRiotId(riotId);
  const showAliasSuggestions = !showPlatforms && suggestionsOpen && !parsedInput.tagLine && aliasSuggestions.length > 0;

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
        const response = await searchAccountAliases(parsed.gameName, platform, 6);
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
    setLastSearch(resolvedRiotID);
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
          setLastSearch(resolvedRiotID);
          setSuggestionsOpen(false);
          onSearch(alias.gameName, alias.tagLine, alias.platform || platform);
          return;
        }
        if (alias.status === 'ambiguous') {
          setValidationError(`Tag required. Multiple saved Riot IDs match '${parsed.gameName}'. Use Name#Tag.`);
          return;
        }
        setValidationError(`Tag required. No unique saved Riot ID for '${parsed.gameName}'. Use Name#Tag.`);
      } catch (aliasError) {
        setValidationError('Tag required. Saved Riot ID lookup failed; use Name#Tag.');
      } finally {
        setAliasLoading(false);
      }
      return;
    }
    setValidationError('');
    setLastSearch(`${parsed.gameName}#${parsed.tagLine}`);
    onSearch(parsed.gameName, parsed.tagLine, platform);
  };

  const lookupMessage = validationError || (error ? lookupErrorMessage(lastSearch || riotId, error.message) : '');
  const pendingMessage = aliasLoading ? 'Checking saved Riot IDs...' : loading ? 'Checking live game...' : '';
  const showLiveGame = Boolean(liveGame && !lookupMessage);

  return (
    <section className={showLiveGame ? 'live-panel has-game' : 'live-panel search-only'}>
      <div className={showLiveGame ? 'search-section compact-search' : 'search-section lookup-console'}>
        {!showLiveGame ? (
          <div className="lookup-console-header">
            <span>Summoner Search</span>
            <strong>Live Match Lookup</strong>
          </div>
        ) : null}
        <div className={validationError ? 'search-bar invalid' : 'search-bar'}>
          <RadioTower className="search-mark" size={22} />
          <input
            value={riotId}
            placeholder="Riot ID, e.g. TWITCH ELOSANTA#1111"
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
          <button className="search-button" onClick={search} title="Find live game" aria-label="Find live game">
            <Search size={19} />
            <span>{showLiveGame ? 'Find' : 'Search'}</span>
          </button>
        </div>
        {showPlatforms && (
          <div className="server-options-row">
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
        )}
        {showAliasSuggestions && (
          <div className="search-autocomplete">
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
        )}
        <div className={pendingMessage || lookupMessage ? 'lookup-status has-message' : 'lookup-status'} aria-live="polite">
          {pendingMessage && (
            <div className="search-message-card checking">
              <LoaderCircle size={16} aria-hidden="true" />
              <span>{pendingMessage}</span>
            </div>
          )}
          {lookupMessage && (
            <div className="search-message-card error">
              <CircleAlert size={16} aria-hidden="true" />
              <span>{lookupMessage}</span>
            </div>
          )}
        </div>
      </div>
      {showLiveGame && liveGame && (
        <LiveMatchups
          liveGame={liveGame}
          champions={champions}
          items={items}
          spells={spells}
          runes={runes}
        />
      )}
    </section>
  );
}

function lookupErrorMessage(riotId: string, message: string) {
  const normalized = message.toLowerCase();
  if (normalized.includes('not currently in a live game') || normalized.includes('riot id not found')) {
    return `Summoner '${riotId.trim()}' is not currently in a live match`;
  }
  return message;
}

function parseRiotId(value: string) {
  const trimmed = value.trim();
  const separator = trimmed.lastIndexOf('#');
  if (separator === -1) {
    return { gameName: trimmed, tagLine: '' };
  }
  return {
    gameName: trimmed.slice(0, separator).trim(),
    tagLine: trimmed.slice(separator + 1).trim(),
  };
}

function platformLabel(value: string) {
  return platforms.find((candidate) => candidate.value === value)?.label ?? value;
}
