import { useEffect, useState } from 'react';
import type { Champion, ChampionData } from '../../api/types';
import { championSplashUrl } from '../../lib/staticData';

const carouselPans = ['pan-east', 'pan-west', 'pan-rise', 'pan-fall', 'pan-northeast', 'pan-southwest'];

type ChampionSplashSlide = {
  champion: Champion;
  panClass: string;
  cycle: number;
};

type ChampionSplashCarouselState = {
  active?: ChampionSplashSlide;
  previous?: ChampionSplashSlide;
};

export function useChampionSplashCarousel(champions: Champion[], key: string): ChampionSplashCarouselState {
  const [state, setState] = useState<ChampionSplashCarouselState>({});
  const championKey = champions.map((champion) => champion.key).join(':');

  useEffect(() => {
    if (!champions.length) {
      setState({});
      return undefined;
    }

    const firstChampion = champions[randomIndex(champions.length)];
    setState({ active: splashSlide(firstChampion, 0, key) });
    if (champions.length <= 1) {
      return undefined;
    }

    const intervalMs = 11200 + randomIndex(1600);
    let cycle = 0;
    const timer = window.setInterval(() => {
      setState((current) => {
        const currentKey = current.active?.champion.key;
        const candidates = champions.filter((champion) => champion.key !== currentKey);
        const nextChampion = candidates[randomIndex(candidates.length)] ?? champions[0];
        cycle += 1;
        return {
          active: splashSlide(nextChampion, cycle, key),
          previous: current.active,
        };
      });
    }, intervalMs);

    return () => window.clearInterval(timer);
  }, [championKey, key]);

  return state;
}

export function WinConditionCardArt({ carousel, champions }: { carousel: ChampionSplashCarouselState; champions?: ChampionData }) {
  return (
    <div className="win-condition-card-art" aria-hidden="true">
      {carousel.previous ? <WinConditionArtLayer slide={carousel.previous} champions={champions} state="exiting" /> : null}
      {carousel.active ? <WinConditionArtLayer slide={carousel.active} champions={champions} state="active" /> : null}
    </div>
  );
}

export function ControlPairArt({
  carryCarousel,
  protectorCarousel,
  champions,
}: {
  carryCarousel: ChampionSplashCarouselState;
  protectorCarousel: ChampionSplashCarouselState;
  champions?: ChampionData;
}) {
  return (
    <div className="control-pair-art" aria-hidden="true">
      <div className="control-pair-side carry">
        {carryCarousel.previous ? <WinConditionArtLayer slide={carryCarousel.previous} champions={champions} state="exiting" /> : null}
        {carryCarousel.active ? <WinConditionArtLayer slide={carryCarousel.active} champions={champions} state="active" /> : null}
      </div>
      <div className="control-pair-side protector">
        {protectorCarousel.previous ? <WinConditionArtLayer slide={protectorCarousel.previous} champions={champions} state="exiting" /> : null}
        {protectorCarousel.active ? <WinConditionArtLayer slide={protectorCarousel.active} champions={champions} state="active" /> : null}
      </div>
      <span className="control-pair-blend" />
    </div>
  );
}

function WinConditionArtLayer({
  slide,
  champions,
  state,
}: {
  slide: ChampionSplashSlide;
  champions?: ChampionData;
  state: 'active' | 'exiting';
}) {
  const source = championSplashUrl(champions, Number(slide.champion.key));
  if (!source) {
    return null;
  }
  return (
    <img
      key={`${state}-${slide.cycle}-${slide.champion.id}`}
      className={`win-condition-art-layer ${state} ${slide.panClass}`}
      src={source}
      alt=""
    />
  );
}

function splashSlide(champion: Champion, cycle: number, key: string): ChampionSplashSlide {
  const panIndex = (hashText(`${key}:${champion.id}:${cycle}`) + cycle) % carouselPans.length;
  return {
    champion,
    cycle,
    panClass: carouselPans[panIndex],
  };
}

function randomIndex(count: number) {
  return Math.floor(Math.random() * Math.max(1, count));
}

function hashText(value: string) {
  let hash = 0;
  for (let index = 0; index < value.length; index += 1) {
    hash = (hash * 31 + value.charCodeAt(index)) >>> 0;
  }
  return hash;
}
