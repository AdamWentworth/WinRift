import { ArrowRight } from 'lucide-react';
import type { CSSProperties } from 'react';
import type { Champion, ChampionData } from '../../api/types';
import { conditionIconUrl, type WinConditionDefinition, type WinConditionKey } from '../../lib/winConditions';
import { ExampleChampionCarousel } from './ExampleChampionCarousel';
import { matchingChampions } from './helpers';
import { ControlPairArt, useChampionSplashCarousel, WinConditionCardArt } from './WinConditionArt';

export function WinConditionGuideCard({
  definition,
  champions,
  onSelectChampion,
  onSelectCondition,
}: {
  definition: WinConditionDefinition;
  champions?: ChampionData;
  onSelectChampion: (champion: Champion) => void;
  onSelectCondition: (condition: WinConditionKey) => void;
}) {
  const exampleChampions = matchingChampions(champions, definition.examples);
  const carryChampions = matchingChampions(champions, definition.carryExamples ?? []);
  const protectorChampions = matchingChampions(champions, definition.protectorExamples ?? []);
  const artCarousel = useChampionSplashCarousel(exampleChampions, definition.key);
  const carryCarousel = useChampionSplashCarousel(carryChampions, `${definition.key}-carry`);
  const protectorCarousel = useChampionSplashCarousel(protectorChampions, `${definition.key}-protector`);
  const isControl = definition.key === 'Control';

  return (
    <article
      className={`win-condition-guide-card${isControl ? ' spotlight control-pair' : ''}`}
      id={definition.key}
      style={{ '--condition-accent': definition.accent } as CSSProperties}
    >
      {isControl ? (
        <ControlPairArt
          carryCarousel={carryCarousel}
          protectorCarousel={protectorCarousel}
          champions={champions}
        />
      ) : (
        <WinConditionCardArt carousel={artCarousel} champions={champions} />
      )}
      <header>
        <img src={conditionIconUrl(definition.key)} alt="" />
        <div>
          <span className="win-condition-card-subtitle">{definition.shortLabel}</span>
          <h3>{definition.label}</h3>
        </div>
      </header>
      <p className="win-condition-main-copy">{definition.plainEnglish}</p>
      <button className="win-condition-guide-link" onClick={() => onSelectCondition(definition.key)} type="button">
        Open {definition.label} guide
        <ArrowRight size={14} aria-hidden="true" />
      </button>
      {isControl ? (
        <div className="win-condition-control-examples">
          <ExampleChampionCarousel
            championNames={definition.carryExamples ?? []}
            champions={champions}
            conditionLabel={definition.label}
            label="Carry examples"
            onSelectChampion={onSelectChampion}
          />
          <ExampleChampionCarousel
            championNames={definition.protectorExamples ?? []}
            champions={champions}
            conditionLabel={definition.label}
            label="Protector examples"
            onSelectChampion={onSelectChampion}
          />
        </div>
      ) : (
        <ExampleChampionCarousel
          championNames={definition.examples}
          champions={champions}
          conditionLabel={definition.label}
          label="Example champions"
          onSelectChampion={onSelectChampion}
        />
      )}
    </article>
  );
}
