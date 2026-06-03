import { useEffect, useMemo, useState } from 'react';
import type { Champion, ChampionData, ChampionSplashData } from '../api/types';
import { championByKey, championList } from '../lib/staticData';

type GlobalBackgroundSlide = {
  src: string;
  title: string;
  position: string;
  panClass: string;
};

const fallbackGlobalBackgroundSlides: GlobalBackgroundSlide[] = [
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

type GlobalBackgroundSlideState = {
  deck: GlobalBackgroundSlide[];
  active: GlobalBackgroundSlide;
  previous?: GlobalBackgroundSlide;
  nextIndex: number;
  cycle: number;
};

type Props = {
  champions?: ChampionData;
  championSplashes?: ChampionSplashData;
  championScopeId?: number;
  championScopeIds?: number[];
  strictChampionScope?: boolean;
};

export function GlobalBackgroundStage({ champions, championSplashes, championScopeId, championScopeIds, strictChampionScope }: Props) {
  const scopeKey = championScopeIds?.join(':') ?? '';
  const slidePool = useMemo(() => buildGlobalBackgroundSplashPool(champions, championSplashes, championScopeId, championScopeIds, strictChampionScope), [champions, championScopeId, championSplashes, scopeKey, strictChampionScope]);
  const [slideState, setSlideState] = useState(() => initialGlobalBackgroundSlideState(fallbackGlobalBackgroundSlides));

  useEffect(() => {
    setSlideState(initialGlobalBackgroundSlideState(slidePool));
  }, [slidePool]);

  useEffect(() => {
    if (slidePool.length <= 1) {
      return undefined;
    }
    const interval = window.setInterval(() => {
      setSlideState((current) => nextGlobalBackgroundSlideState(current, slidePool));
    }, 10500);
    return () => window.clearInterval(interval);
  }, [slidePool]);

  if (!slidePool.length) {
    return null;
  }

  return (
    <div className="global-art-stage" aria-hidden="true">
      {slideState.previous ? (
        <GlobalBackgroundArtSlide
          key={`previous-${slideState.cycle}-${slideState.previous.src}`}
          slide={slideState.previous}
          state="exiting"
        />
      ) : null}
      <GlobalBackgroundArtSlide
        key={`active-${slideState.cycle}-${slideState.active.src}`}
        slide={slideState.active}
        state="active"
      />
    </div>
  );
}

function GlobalBackgroundArtSlide({ slide, state }: { slide: GlobalBackgroundSlide; state: 'active' | 'exiting' }) {
  return (
    <div className={`global-art-slide ${state} ${slide.panClass}`}>
      <img src={slide.src} alt="" title={slide.title} style={{ objectPosition: slide.position }} />
    </div>
  );
}

function buildGlobalBackgroundSplashPool(champions?: ChampionData, championSplashes?: ChampionSplashData, championScopeId?: number, championScopeIds?: number[], strictChampionScope?: boolean): GlobalBackgroundSlide[] {
  const scopedChampion = championScopeId ? championByKey(champions, championScopeId) : undefined;
  if (scopedChampion) {
    const scopedSplashes = championSplashes?.data.filter((splash) => splash.championId === scopedChampion.id) ?? [];
    if (scopedSplashes.length) {
      return mapSplashSlides(scopedSplashes);
    }
    return [{
      src: `https://ddragon.leagueoflegends.com/cdn/img/champion/splash/${scopedChampion.id}_0.jpg`,
      title: scopedChampion.name,
      position: globalBackgroundSplashPosition(scopedChampion.id),
      panClass: globalBackgroundSplashPan(scopedChampion.id, 0),
    }];
  }
  const scopedChampions = uniqueNumbers(championScopeIds ?? [])
    .map((championId) => championByKey(champions, championId))
    .filter((champion): champion is Champion => Boolean(champion));
  if (scopedChampions.length) {
    const scopedChampionIds = new Set(scopedChampions.map((champion) => champion.id));
    const scopedSplashes = championSplashes?.data.filter((splash) => scopedChampionIds.has(splash.championId)) ?? [];
    if (scopedSplashes.length) {
      return mapSplashSlides(scopedSplashes);
    }
    return scopedChampions.map((champion, index) => ({
      src: `https://ddragon.leagueoflegends.com/cdn/img/champion/splash/${champion.id}_0.jpg`,
      title: champion.name,
      position: globalBackgroundSplashPosition(champion.id),
      panClass: globalBackgroundSplashPan(champion.id, index),
    }));
  }
  if (strictChampionScope) {
    return [];
  }
  if (championSplashes?.data.length) {
    return mapSplashSlides(championSplashes.data);
  }
  const championsByName = championList(champions);
  if (!championsByName.length) {
    return fallbackGlobalBackgroundSlides;
  }
  return championsByName.map((champion, index) => ({
    src: `https://ddragon.leagueoflegends.com/cdn/img/champion/splash/${champion.id}_0.jpg`,
    title: champion.name,
    position: globalBackgroundSplashPosition(champion.id),
    panClass: globalBackgroundSplashPan(champion.id, index),
  }));
}

function mapSplashSlides(splashes: ChampionSplashData['data']): GlobalBackgroundSlide[] {
  return splashes.map((splash, index) => {
    const key = `${splash.championId}-${splash.skinNumber}`;
    return {
      src: splash.src,
      title: splash.skinNumber === 0 ? splash.championName : splash.skinName,
      position: globalBackgroundSplashPosition(key),
      panClass: globalBackgroundSplashPan(key, index),
    };
  });
}

function initialGlobalBackgroundSlideState(slides: GlobalBackgroundSlide[]): GlobalBackgroundSlideState {
  const deck = shuffleGlobalBackgroundSlides(slides);
  return {
    deck,
    active: deck[0] ?? fallbackGlobalBackgroundSlides[0],
    nextIndex: deck.length > 1 ? 1 : 0,
    cycle: 0,
  };
}

function nextGlobalBackgroundSlideState(current: GlobalBackgroundSlideState, slidePool: GlobalBackgroundSlide[]): GlobalBackgroundSlideState {
  let deck = current.deck;
  let nextIndex = current.nextIndex;
  if (nextIndex >= deck.length) {
    deck = shuffleGlobalBackgroundSlides(slidePool, current.active.src);
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

function shuffleGlobalBackgroundSlides(slides: GlobalBackgroundSlide[], avoidFirstSrc?: string): GlobalBackgroundSlide[] {
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

function globalBackgroundSplashPosition(championId: string) {
  const positions = ['center 34%', 'center 38%', 'center 42%', '44% 38%', '56% 38%', '50% 46%'];
  return positions[hashText(championId) % positions.length];
}

function globalBackgroundSplashPan(championId: string, index: number) {
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

function uniqueNumbers(values: number[]) {
  const seen = new Set<number>();
  return values.filter((value) => {
    if (!value || seen.has(value)) {
      return false;
    }
    seen.add(value);
    return true;
  });
}
