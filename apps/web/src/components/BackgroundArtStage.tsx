import { useEffect, useMemo, useState } from 'react';
import type { ChampionData, ChampionSplashData } from '../api/types';
import { championByKey, championList } from '../lib/staticData';

type BackgroundArtSlide = {
  src: string;
  title: string;
  position: string;
  panClass: string;
};

const fallbackBackgroundSlides: BackgroundArtSlide[] = [
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

type BackgroundSlideState = {
  deck: BackgroundArtSlide[];
  active: BackgroundArtSlide;
  previous?: BackgroundArtSlide;
  nextIndex: number;
  cycle: number;
};

type Props = {
  champions?: ChampionData;
  championSplashes?: ChampionSplashData;
  championId?: number;
};

export function BackgroundArtStage({ champions, championSplashes, championId }: Props) {
  const slidePool = useMemo(() => buildBackgroundSplashPool(champions, championSplashes, championId), [champions, championId, championSplashes]);
  const [slideState, setSlideState] = useState(() => initialBackgroundSlideState(fallbackBackgroundSlides));

  useEffect(() => {
    setSlideState(initialBackgroundSlideState(slidePool));
  }, [slidePool]);

  useEffect(() => {
    if (slidePool.length <= 1) {
      return undefined;
    }
    const interval = window.setInterval(() => {
      setSlideState((current) => nextBackgroundSlideState(current, slidePool));
    }, 10500);
    return () => window.clearInterval(interval);
  }, [slidePool]);

  return (
    <div className="home-art-stage" aria-hidden="true">
      {slideState.previous ? (
        <BackgroundArtSlide
          key={`previous-${slideState.cycle}-${slideState.previous.src}`}
          slide={slideState.previous}
          state="exiting"
        />
      ) : null}
      <BackgroundArtSlide
        key={`active-${slideState.cycle}-${slideState.active.src}`}
        slide={slideState.active}
        state="active"
      />
    </div>
  );
}

function BackgroundArtSlide({ slide, state }: { slide: BackgroundArtSlide; state: 'active' | 'exiting' }) {
  return (
    <div className={`home-art-slide ${state} ${slide.panClass}`}>
      <img src={slide.src} alt="" title={slide.title} style={{ objectPosition: slide.position }} />
    </div>
  );
}

function buildBackgroundSplashPool(champions?: ChampionData, championSplashes?: ChampionSplashData, championId?: number): BackgroundArtSlide[] {
  const scopedChampion = championId ? championByKey(champions, championId) : undefined;
  if (scopedChampion) {
    const scopedSplashes = championSplashes?.data.filter((splash) => splash.championId === scopedChampion.id) ?? [];
    if (scopedSplashes.length) {
      return mapSplashSlides(scopedSplashes);
    }
    return [{
      src: `https://ddragon.leagueoflegends.com/cdn/img/champion/splash/${scopedChampion.id}_0.jpg`,
      title: scopedChampion.name,
      position: backgroundSplashPosition(scopedChampion.id),
      panClass: backgroundSplashPan(scopedChampion.id, 0),
    }];
  }
  if (championSplashes?.data.length) {
    return mapSplashSlides(championSplashes.data);
  }
  const championsByName = championList(champions);
  if (!championsByName.length) {
    return fallbackBackgroundSlides;
  }
  return championsByName.map((champion, index) => ({
    src: `https://ddragon.leagueoflegends.com/cdn/img/champion/splash/${champion.id}_0.jpg`,
    title: champion.name,
    position: backgroundSplashPosition(champion.id),
    panClass: backgroundSplashPan(champion.id, index),
  }));
}

function mapSplashSlides(splashes: ChampionSplashData['data']): BackgroundArtSlide[] {
  return splashes.map((splash, index) => {
    const key = `${splash.championId}-${splash.skinNumber}`;
    return {
      src: splash.src,
      title: splash.skinNumber === 0 ? splash.championName : splash.skinName,
      position: backgroundSplashPosition(key),
      panClass: backgroundSplashPan(key, index),
    };
  });
}

function initialBackgroundSlideState(slides: BackgroundArtSlide[]): BackgroundSlideState {
  const deck = shuffleBackgroundSlides(slides);
  return {
    deck,
    active: deck[0] ?? fallbackBackgroundSlides[0],
    nextIndex: deck.length > 1 ? 1 : 0,
    cycle: 0,
  };
}

function nextBackgroundSlideState(current: BackgroundSlideState, slidePool: BackgroundArtSlide[]): BackgroundSlideState {
  let deck = current.deck;
  let nextIndex = current.nextIndex;
  if (nextIndex >= deck.length) {
    deck = shuffleBackgroundSlides(slidePool, current.active.src);
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

function shuffleBackgroundSlides(slides: BackgroundArtSlide[], avoidFirstSrc?: string): BackgroundArtSlide[] {
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

function backgroundSplashPosition(championId: string) {
  const positions = ['center 34%', 'center 38%', 'center 42%', '44% 38%', '56% 38%', '50% 46%'];
  return positions[hashText(championId) % positions.length];
}

function backgroundSplashPan(championId: string, index: number) {
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
