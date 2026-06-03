import { ArrowLeft } from 'lucide-react';
import { useMemo } from 'react';
import type { CSSProperties } from 'react';
import type { Champion, ChampionData } from '../../api/types';
import { conditionIconUrl, FLEX_ARCHETYPE_DEFINITION, FLEX_ARCHETYPE_DETAIL, type WinConditionKey } from '../../lib/winConditions';
import { ExampleChampionCarousel } from './ExampleChampionCarousel';
import { orderedWinConditionDefinitions } from './helpers';
import { SectionKicker, visualForHeader } from './SectionKicker';

export function FlexArchetypePage({
  champions,
  onBack,
  onSelectChampion,
  onSelectCondition,
}: {
  champions?: ChampionData;
  onBack: () => void;
  onSelectChampion: (champion: Champion) => void;
  onSelectCondition: (condition: WinConditionKey) => void;
}) {
  const orderedDefinitions = useMemo(() => orderedWinConditionDefinitions(), []);
  const definition = FLEX_ARCHETYPE_DEFINITION;
  const detail = FLEX_ARCHETYPE_DETAIL;

  return (
    <section
      className="win-condition-detail-page flex-archetype-page"
      style={{ '--condition-accent': definition.accent } as CSSProperties}
    >
      <div className="win-condition-detail-hero flex-archetype-hero global-window">
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
        <div className="win-condition-detail-nav flex-archetype-nav" aria-label="Strategy guides">
          {orderedDefinitions.map((candidate) => (
            <button
              key={candidate.key}
              onClick={() => onSelectCondition(candidate.key)}
              style={{ '--condition-accent': candidate.accent } as CSSProperties}
              type="button"
            >
              <img src={conditionIconUrl(candidate.key)} alt="" />
              <span>{candidate.label}</span>
            </button>
          ))}
          <button aria-current="page" style={{ '--condition-accent': definition.accent } as CSSProperties} type="button">
            <img src={conditionIconUrl(definition.key)} alt="" />
            <span>{definition.label}</span>
          </button>
        </div>
      </div>

      <div className="flex-archetype-note">
        <SectionKicker {...visualForHeader('Special Archetype')} label="Special Archetype" />
        <p>
          Flex is intentionally separate from the five team win-condition axes. A team still wins through Split Push,
          Pick, Siege, Control, or Team Fight. Flex explains why certain champions can connect those plans, disguise
          draft intent, or help the team adapt when the best path changes.
        </p>
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
        <DetailList title="Flex Signals" items={detail.signals} />
        <DetailList title="Adaptive Pattern" items={detail.playPattern} />
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

      <ExampleChampionCarousel
        championNames={definition.examples}
        champions={champions}
        conditionLabel={definition.label}
        label="Flex champion examples"
        onSelectChampion={onSelectChampion}
      />
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
