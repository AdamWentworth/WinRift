import { CircleAlert, LoaderCircle, RadioTower, Search } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import { resolveAccountAlias, searchAccountAliases } from '../api/client';
import type { AccountAliasMatch, ChampionData, ChampionSplashData, ItemData, LiveGame, RuneData, SummonerSpellData } from '../api/types';
import { championList } from '../lib/staticData';
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

type HomeSplashSlide = {
  src: string;
  title: string;
  position: string;
  panClass: string;
};

const fallbackHomeSplashSlides: HomeSplashSlide[] = [
  {
    src: 'https://ddragon.leagueoflegends.com/cdn/img/champion/splash/Akali_0.jpg',
    title: 'Akali',
    position: 'center 38%',
    panClass: 'pan-east',
  },
  {
    src: 'https://ddragon.leagueoflegends.com/cdn/img/champion/splash/Ashe_0.jpg',
    title: 'Ashe',
    position: 'center 40%',
    panClass: 'pan-west',
  },
  {
    src: 'https://ddragon.leagueoflegends.com/cdn/img/champion/splash/Ekko_0.jpg',
    title: 'Ekko',
    position: 'center 36%',
    panClass: 'pan-rise',
  },
  {
    src: 'https://ddragon.leagueoflegends.com/cdn/img/champion/splash/Thresh_0.jpg',
    title: 'Thresh',
    position: 'center 42%',
    panClass: 'pan-fall',
  },
  {
    src: 'https://ddragon.leagueoflegends.com/cdn/img/champion/splash/AurelionSol_0.jpg',
    title: 'Aurelion Sol',
    position: 'center 46%',
    panClass: 'pan-northeast',
  },
  {
    src: 'https://ddragon.leagueoflegends.com/cdn/img/champion/splash/Leona_0.jpg',
    title: 'Leona',
    position: 'center 38%',
    panClass: 'pan-southwest',
  },
];

type HomeSlideState = {
  deck: HomeSplashSlide[];
  active: HomeSplashSlide;
  previous?: HomeSplashSlide;
  nextIndex: number;
  cycle: number;
};

type Props = {
  champions?: ChampionData;
  championSplashes?: ChampionSplashData;
  items?: ItemData;
  spells?: SummonerSpellData;
  runes?: RuneData;
  liveGame?: LiveGame;
  loading: boolean;
  error?: Error | null;
  onSearch: (gameName: string, tagLine: string, platform: string) => void;
};

export function LiveMatchPanel({ champions, championSplashes, items, spells, runes, liveGame, loading, error, onSearch }: Props) {
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
      <HomeArtStage champions={champions} championSplashes={championSplashes} />
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

function HomeArtStage({ champions, championSplashes }: { champions?: ChampionData; championSplashes?: ChampionSplashData }) {
  const slidePool = useMemo(() => buildHomeSplashPool(champions, championSplashes), [champions, championSplashes]);
  const [slideState, setSlideState] = useState(() => initialHomeSlideState(fallbackHomeSplashSlides));

  useEffect(() => {
    setSlideState(initialHomeSlideState(slidePool));
  }, [slidePool]);

  useEffect(() => {
    if (slidePool.length <= 1) {
      return undefined;
    }
    const interval = window.setInterval(() => {
      setSlideState((current) => nextHomeSlideState(current, slidePool));
    }, 10500);
    return () => window.clearInterval(interval);
  }, [slidePool]);

  return (
    <div className="home-art-stage" aria-hidden="true">
      {slideState.previous ? (
        <HomeArtSlide
          key={`previous-${slideState.cycle}-${slideState.previous.src}`}
          slide={slideState.previous}
          state="exiting"
        />
      ) : null}
      <HomeArtSlide
        key={`active-${slideState.cycle}-${slideState.active.src}`}
        slide={slideState.active}
        state="active"
      />
    </div>
  );
}

function HomeArtSlide({ slide, state }: { slide: HomeSplashSlide; state: 'active' | 'exiting' }) {
  return (
    <div className={`home-art-slide ${state} ${slide.panClass}`}>
      <img src={slide.src} alt="" title={slide.title} style={{ objectPosition: slide.position }} />
    </div>
  );
}

function buildHomeSplashPool(champions?: ChampionData, championSplashes?: ChampionSplashData): HomeSplashSlide[] {
  if (championSplashes?.data.length) {
    return championSplashes.data.map((splash, index) => {
      const key = `${splash.championId}-${splash.skinNumber}`;
      return {
        src: splash.src,
        title: splash.skinNumber === 0 ? splash.championName : splash.skinName,
        position: homeSplashPosition(key),
        panClass: homeSplashPan(key, index),
      };
    });
  }
  const championsByName = championList(champions);
  if (!championsByName.length) {
    return fallbackHomeSplashSlides;
  }
  return championsByName.map((champion, index) => ({
    src: `https://ddragon.leagueoflegends.com/cdn/img/champion/splash/${champion.id}_0.jpg`,
    title: champion.name,
    position: homeSplashPosition(champion.id),
    panClass: homeSplashPan(champion.id, index),
  }));
}

function initialHomeSlideState(slides: HomeSplashSlide[]): HomeSlideState {
  const deck = shuffleHomeSlides(slides);
  return {
    deck,
    active: deck[0] ?? fallbackHomeSplashSlides[0],
    nextIndex: deck.length > 1 ? 1 : 0,
    cycle: 0,
  };
}

function nextHomeSlideState(current: HomeSlideState, slidePool: HomeSplashSlide[]): HomeSlideState {
  let deck = current.deck;
  let nextIndex = current.nextIndex;
  if (nextIndex >= deck.length) {
    deck = shuffleHomeSlides(slidePool, current.active.src);
    nextIndex = 0;
  }
  const active = deck[nextIndex] ?? current.active;
  return {
    deck,
    active,
    previous: current.active,
    nextIndex: nextIndex + 1,
    cycle: current.cycle + 1,
  };
}

function shuffleHomeSlides(slides: HomeSplashSlide[], avoidFirstSrc?: string): HomeSplashSlide[] {
  const shuffled = [...slides];
  for (let index = shuffled.length - 1; index > 0; index -= 1) {
    const swapIndex = Math.floor(Math.random() * (index + 1));
    [shuffled[index], shuffled[swapIndex]] = [shuffled[swapIndex], shuffled[index]];
  }
  if (avoidFirstSrc && shuffled.length > 1 && shuffled[0]?.src === avoidFirstSrc) {
    const swapIndex = 1 + Math.floor(Math.random() * (shuffled.length - 1));
    [shuffled[0], shuffled[swapIndex]] = [shuffled[swapIndex], shuffled[0]];
  }
  return shuffled;
}

function homeSplashPosition(championId: string) {
  const positions = ['center 34%', 'center 38%', 'center 42%', '44% 38%', '56% 38%', '50% 46%'];
  return positions[hashText(championId) % positions.length];
}

function homeSplashPan(championId: string, index: number) {
  const pans = ['pan-east', 'pan-west', 'pan-rise', 'pan-fall', 'pan-northeast', 'pan-southwest'];
  return pans[(hashText(championId) + index) % pans.length];
}

function hashText(value: string) {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) {
    hash = (hash * 31 + value.charCodeAt(index)) >>> 0;
  }
  return hash;
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
