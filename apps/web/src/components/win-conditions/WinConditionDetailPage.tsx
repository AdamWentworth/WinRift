import { ArrowLeft } from 'lucide-react';
import { useMemo } from 'react';
import type { CSSProperties } from 'react';
import type { Champion, ChampionData } from '../../api/types';
import { conditionIconUrl, winConditionDetail, type WinConditionKey } from '../../lib/winConditions';
import { ExampleChampionCarousel } from './ExampleChampionCarousel';
import { matchingChampions, orderedWinConditionDefinitions, winConditionDefinitionByKey } from './helpers';
import { SectionKicker, visualForHeader } from './SectionKicker';
import { ControlPairArt, useChampionSplashCarousel, WinConditionCardArt } from './WinConditionArt';

export function WinConditionDetailPage({
  champions,
  condition,
  onBack,
  onSelectChampion,
  onSelectCondition,
}: {
  champions?: ChampionData;
  condition: WinConditionKey;
  onBack: () => void;
  onSelectChampion: (champion: Champion) => void;
  onSelectCondition: (condition: WinConditionKey) => void;
}) {
  const definition = winConditionDefinitionByKey(condition);
  const detail = winConditionDetail(condition);
  const orderedDefinitions = useMemo(() => orderedWinConditionDefinitions(), []);
  const exampleChampions = matchingChampions(champions, definition.examples);
  const carryChampions = matchingChampions(champions, definition.carryExamples ?? []);
  const protectorChampions = matchingChampions(champions, definition.protectorExamples ?? []);
  const artCarousel = useChampionSplashCarousel(exampleChampions, `${condition}-detail`);
  const carryCarousel = useChampionSplashCarousel(carryChampions, `${condition}-detail-carry`);
  const protectorCarousel = useChampionSplashCarousel(protectorChampions, `${condition}-detail-protector`);
  const isControl = condition === 'Control';

  return (
    <section className="win-condition-detail-page" style={{ '--condition-accent': definition.accent } as CSSProperties}>
      <div className={`win-condition-detail-hero${isControl ? ' control-pair' : ''}`}>
        {isControl ? (
          <ControlPairArt carryCarousel={carryCarousel} protectorCarousel={protectorCarousel} champions={champions} />
        ) : (
          <WinConditionCardArt carousel={artCarousel} champions={champions} />
        )}
        <div className="win-condition-detail-hero-copy">
          <button className="win-condition-back-button" onClick={onBack} type="button">
            <ArrowLeft size={15} aria-hidden="true" />
            Win Conditions
          </button>
          <div className="win-condition-detail-title-row">
            <img src={conditionIconUrl(definition.key)} alt="" />
            <div>
              <span className="win-condition-detail-subtitle">{definition.shortLabel}</span>
              <h2>{definition.label}</h2>
            </div>
          </div>
          <p>{detail.thesis}</p>
        </div>
        <div className="win-condition-detail-nav" aria-label="Other win conditions">
          {orderedDefinitions.map((candidate) => (
            <button
              aria-current={candidate.key === condition ? 'page' : undefined}
              key={candidate.key}
              onClick={() => onSelectCondition(candidate.key)}
              style={{ '--condition-accent': candidate.accent } as CSSProperties}
              type="button"
            >
              <img src={conditionIconUrl(candidate.key)} alt="" />
              <span>{candidate.label}</span>
            </button>
          ))}
        </div>
      </div>

      <div className="win-condition-detail-layout">
        <div className="win-condition-detail-main">
          {detail.sections.map((section) => (
            <article className="win-condition-detail-section" key={section.title}>
              <SectionKicker {...visualForHeader(section.title)} label={section.title} />
              <p>{section.body}</p>
            </article>
          ))}
        </div>
        <aside className="win-condition-detail-aside">
          <ReadDetail label="Map Pattern" text={definition.mapPattern} />
          <ReadDetail label="Team Needs" text={definition.teamNeeds} />
          <ReadDetail label="Common Failure" text={definition.commonFailure} />
        </aside>
      </div>

      <div className="win-condition-detail-list-grid">
        <DetailList title="Composition Signals" items={detail.signals} />
        <DetailList title="Play Pattern" items={detail.playPattern} />
        <DetailList title="Failure Checks" items={detail.failureChecks} />
      </div>

      <div className="win-condition-detail-matchups">
        <article>
          <SectionKicker {...visualForHeader('Usually Good Into')} label="Usually Good Into" />
          <p>{detail.goodInto}</p>
        </article>
        <article>
          <SectionKicker {...visualForHeader('Usually Struggles Into')} label="Usually Struggles Into" />
          <p>{detail.strugglesInto}</p>
        </article>
        <article>
          <SectionKicker {...visualForHeader('Timing Read')} label="Timing Read" />
          <p>{detail.timing}</p>
        </article>
        <article>
          <SectionKicker {...visualForHeader('Live Page Interpretation')} label="Live Page Interpretation" />
          <p>{detail.liveRead}</p>
        </article>
      </div>

      {isControl ? (
        <div className="win-condition-control-examples detail">
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
          label={`${definition.label} champion examples`}
          onSelectChampion={onSelectChampion}
        />
      )}
    </section>
  );
}

function DetailList({ title, items }: { title: string; items: string[] }) {
  return (
    <article>
      <SectionKicker {...visualForHeader(title)} label={title} />
      <ul>
        {items.map((item) => (
          <li key={item}>{item}</li>
        ))}
      </ul>
    </article>
  );
}

function ReadDetail({ label, text }: { label: string; text: string }) {
  return (
    <div>
      <SectionKicker {...visualForHeader(label)} label={label} />
      <p>{text}</p>
    </div>
  );
}
