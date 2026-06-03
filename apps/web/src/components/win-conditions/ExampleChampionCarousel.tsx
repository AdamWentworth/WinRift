import { ArrowRight, ChevronLeft, ChevronRight } from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import type { Champion, ChampionData } from '../../api/types';
import { championImageUrl } from '../../lib/staticData';
import { championByDisplayName } from './helpers';
import { SectionKicker, visualForHeader } from './SectionKicker';

const examplePageSize = 5;

export function ExampleChampionCarousel({
  championNames,
  champions,
  conditionLabel,
  label,
  onSelectChampion,
}: {
  championNames: string[];
  champions?: ChampionData;
  conditionLabel: string;
  label: string;
  onSelectChampion: (champion: Champion) => void;
}) {
  const [exampleOffset, setExampleOffset] = useState(0);
  const namesKey = championNames.join('|');
  const shuffledExamples = useMemo(() => shuffleExamples(championNames), [namesKey]);
  const exampleChampions = championNames
    .map((name) => championByDisplayName(champions, name))
    .filter((champion): champion is Champion => Boolean(champion));
  const lastExampleOffset = lastExamplePageOffset(shuffledExamples.length);
  const pageOffset = Math.min(exampleOffset, lastExampleOffset);
  const visibleExampleNames = shuffledExamples.slice(pageOffset, pageOffset + examplePageSize);
  const canPageExamples = shuffledExamples.length > examplePageSize;
  const exampleRangeStart = visibleExampleNames.length ? pageOffset + 1 : 0;
  const exampleRangeEnd = pageOffset + visibleExampleNames.length;

  useEffect(() => {
    setExampleOffset(0);
  }, [namesKey]);

  function shiftExamplePage(direction: -1 | 1) {
    setExampleOffset((current) => {
      const currentPage = Math.min(current, lastExampleOffset);
      const nextPage = currentPage + direction;
      if (nextPage > lastExampleOffset) {
        return 0;
      }
      if (nextPage < 0) {
        return lastExampleOffset;
      }
      return nextPage;
    });
  }

  return (
    <div className="win-condition-example-strip">
      <div className="win-condition-example-strip-header">
        <SectionKicker {...visualForHeader(label)} label={label} />
        <div className="win-condition-example-controls">
          <button
            aria-label={`Previous ${conditionLabel} ${label.toLowerCase()}`}
            className="win-condition-example-arrow"
            disabled={!canPageExamples}
            onClick={() => shiftExamplePage(-1)}
            type="button"
          >
            <ChevronLeft size={15} aria-hidden="true" />
          </button>
          <em>{exampleRangeStart}-{exampleRangeEnd} / {shuffledExamples.length}</em>
          <button
            aria-label={`Next ${conditionLabel} ${label.toLowerCase()}`}
            className="win-condition-example-arrow"
            disabled={!canPageExamples}
            onClick={() => shiftExamplePage(1)}
            type="button"
          >
            <ChevronRight size={15} aria-hidden="true" />
          </button>
        </div>
      </div>
      <div className="win-condition-example-window">
        {visibleExampleNames.map((name) => {
          const champion = exampleChampions.find((candidate) => candidate.name.toLowerCase() === name.toLowerCase());
          if (!champion) {
            return <span className="win-condition-example-name" key={name}>{name}</span>;
          }
          return (
            <button key={champion.key} type="button" onClick={() => onSelectChampion(champion)}>
              <img src={championImageUrl(champions, Number(champion.key))} alt="" />
              <strong>{champion.name}</strong>
              <ArrowRight size={13} aria-hidden="true" />
            </button>
          );
        })}
      </div>
    </div>
  );
}

function lastExamplePageOffset(count: number) {
  if (count <= examplePageSize) {
    return 0;
  }
  return count - examplePageSize;
}

function shuffleExamples(examples: string[]) {
  const shuffled = [...examples];
  for (let index = shuffled.length - 1; index > 0; index -= 1) {
    const swapIndex = randomIndex(index + 1);
    [shuffled[index], shuffled[swapIndex]] = [shuffled[swapIndex], shuffled[index]];
  }
  return shuffled;
}

function randomIndex(count: number) {
  return Math.floor(Math.random() * Math.max(1, count));
}
