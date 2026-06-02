import { LoaderCircle, Search } from 'lucide-react';
import { useEffect, useState } from 'react';
import { resolveAccountAlias, searchAccountAliases } from '../api/client';
import type { AccountAliasMatch, Champion, ChampionData } from '../api/types';
import { findChampionByLookup, parseRiotId, platformLabel, platforms } from '../lib/lookup';

type HeaderSearchProps = {
  champions?: ChampionData;
  onChampionSearch: (champion: Champion) => void;
  onSearch: (gameName: string, tagLine: string, platform: string) => void;
};

export function HeaderSearch({ champions, onChampionSearch, onSearch }: HeaderSearchProps) {
  const [query, setQuery] = useState('');
  const [platform, setPlatform] = useState('NA1');
  const [showPlatforms, setShowPlatforms] = useState(false);
  const [aliasLoading, setAliasLoading] = useState(false);
  const [aliasSuggestions, setAliasSuggestions] = useState<AccountAliasMatch[]>([]);
  const [suggestionsOpen, setSuggestionsOpen] = useState(false);
  const [status, setStatus] = useState('');
  const selectedPlatform = platforms.find((candidate) => candidate.value === platform) ?? platforms[0];
  const parsedInput = parseRiotId(query);
  const showAliasSuggestions = !showPlatforms && suggestionsOpen && !parsedInput.tagLine && aliasSuggestions.length > 0;

  useEffect(() => {
    const parsed = parseRiotId(query);
    if (parsed.tagLine || parsed.gameName.length < 2) {
      setAliasSuggestions([]);
      setSuggestionsOpen(false);
      return;
    }
    const controller = new AbortController();
    const timer = window.setTimeout(async () => {
      try {
        const response = await searchAccountAliases(parsed.gameName, platform, 5, { signal: controller.signal });
        if (controller.signal.aborted) return;
        setAliasSuggestions(response.matches);
        setSuggestionsOpen(response.matches.length > 0);
      } catch {
        if (!controller.signal.aborted) {
          setAliasSuggestions([]);
          setSuggestionsOpen(false);
        }
      }
    }, 180);
    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [query, platform]);

  const selectAlias = (alias: AccountAliasMatch) => {
    const resolvedRiotID = `${alias.gameName}#${alias.tagLine}`;
    setQuery(resolvedRiotID);
    setStatus('');
    setSuggestionsOpen(false);
    onSearch(alias.gameName, alias.tagLine, alias.platform || platform);
  };

  const submit = async () => {
    const parsed = parseRiotId(query);
    if (!parsed.gameName) {
      setStatus('Enter a champion or Riot ID.');
      return;
    }

    if (!parsed.tagLine && !champions) {
      setStatus('Loading search data...');
      return;
    }

    const championMatch = !parsed.tagLine ? findChampionByLookup(champions, parsed.gameName) : undefined;
    if (championMatch) {
      setStatus('');
      setSuggestionsOpen(false);
      onChampionSearch(championMatch);
      return;
    }

    if (!parsed.tagLine) {
      setStatus('');
      setAliasLoading(true);
      try {
        const alias = await resolveAccountAlias(parsed.gameName, platform);
        if (alias.status === 'found' && alias.gameName && alias.tagLine) {
          const resolvedRiotID = `${alias.gameName}#${alias.tagLine}`;
          setQuery(resolvedRiotID);
          setSuggestionsOpen(false);
          onSearch(alias.gameName, alias.tagLine, alias.platform || platform);
          return;
        }
        if (alias.status === 'ambiguous') {
          setStatus(`Tag needed for ${parsed.gameName}.`);
          return;
        }
        setStatus(`No saved tag for ${parsed.gameName}. Use Name#Tag.`);
      } catch {
        setStatus('Saved Riot ID lookup failed.');
      } finally {
        setAliasLoading(false);
      }
      return;
    }

    setStatus('');
    setSuggestionsOpen(false);
    onSearch(parsed.gameName, parsed.tagLine, platform);
  };

  return (
    <div className="topbar-search" role="search">
      <Search className="topbar-search-icon" size={16} aria-hidden="true" />
      <input
        value={query}
        aria-label="Search WinRift"
        placeholder="Champion or Riot ID"
        onChange={(event) => {
          setQuery(event.target.value);
          setStatus('');
        }}
        onFocus={() => setSuggestionsOpen(aliasSuggestions.length > 0)}
        onKeyDown={(event) => event.key === 'Enter' && submit()}
      />
      <button
        className="topbar-search-platform"
        onClick={() => setShowPlatforms((visible) => !visible)}
        style={{ backgroundColor: selectedPlatform.color }}
        type="button"
      >
        {selectedPlatform.label}
      </button>
      <button className="topbar-search-submit" onClick={submit} type="button" aria-label="Submit header search">
        {aliasLoading ? <LoaderCircle size={16} className="spin-icon" aria-hidden="true" /> : <Search size={16} aria-hidden="true" />}
      </button>
      {showPlatforms ? (
        <div className="topbar-search-platforms">
          {platforms.map((candidate) => (
            <button
              key={candidate.value}
              className={candidate.value === platform ? 'selected' : ''}
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
        <div className="topbar-search-autocomplete">
          {aliasSuggestions.map((alias) => (
            <button
              key={`${alias.platform}:${alias.puuid}:${alias.tagLine}`}
              onClick={() => selectAlias(alias)}
              type="button"
              aria-label={`Use ${alias.gameName}#${alias.tagLine}`}
            >
              <span>{alias.gameName}</span>
              <em>#{alias.tagLine}</em>
              <b>{platformLabel(alias.platform)}</b>
            </button>
          ))}
        </div>
      ) : null}
      <div className={status ? 'topbar-search-status visible' : 'topbar-search-status'} aria-live="polite">
        {status}
      </div>
    </div>
  );
}
