import {
  AlertTriangle,
  ArrowLeft,
  ArrowRight,
  BarChart3,
  BookOpen,
  ChevronLeft,
  ChevronRight,
  Clock3,
  Compass,
  Eye,
  ListChecks,
  MapPinned,
  MessageCircle,
  MousePointer2,
  Network,
  Route,
  Shield,
  ShieldAlert,
  Sparkles,
  Swords,
  Target,
  Users,
} from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
import type { CSSProperties } from 'react';
import type { LucideIcon } from 'lucide-react';
import type { Champion, ChampionData } from '../api/types';
import { championImageUrl, championList, championSplashUrl } from '../lib/staticData';
import {
  conditionIconUrl,
  WIN_CONDITION_DEFINITIONS,
  WIN_CONDITION_PAGE_ORDER,
  winConditionDetail,
  type WinConditionDefinition,
  type WinConditionKey,
} from '../lib/winConditions';

type Props = {
  champions?: ChampionData;
  onSelectCondition: (condition: WinConditionKey) => void;
  onSelectChampion: (champion: Champion) => void;
};

const modelSteps = [
  {
    icon: BookOpen,
    title: 'Champion Identity',
    body: 'Each champion profile splits ten points across Split Push, Pick, Siege, Control, and Team Fight. That keeps every score relative: adding strength to one plan takes weight away from another.',
  },
  {
    icon: BarChart3,
    title: 'Team Shape',
    body: 'A live team profile adds the five champion profiles together. The highest axis becomes the default read, while close secondary axes stay selectable when the comp can realistically pivot.',
  },
  {
    icon: MousePointer2,
    title: 'Matchup Read',
    body: 'WinRift compares your selected strategy into the enemy strategy. If the enemy is clearly playing through something else, switch their strategy and the read updates.',
  },
  {
    icon: ShieldAlert,
    title: 'Evidence, Not Orders',
    body: 'Historical winrates and confidence scores explain what has happened in collected games. They are context, not a command to force a strategy your champions cannot actually execute.',
  },
];

const carouselPans = ['pan-east', 'pan-west', 'pan-rise', 'pan-fall', 'pan-northeast', 'pan-southwest'];
const examplePageSize = 5;
const sectionVisuals: Record<string, HeaderVisual> = {
  'Carry examples': { emoji: '💥', icon: Target },
  'Common Failure': { emoji: '⚠️', icon: AlertTriangle },
  'Composition Signals': { emoji: '🧩', icon: Network },
  'Example champions': { emoji: '⭐', icon: Sparkles },
  'Failure Checks': { emoji: '🚧', icon: ShieldAlert },
  'How It Wins': { emoji: '⚔️', icon: Swords },
  'How To Read It': { emoji: '👁️', icon: Eye },
  'How To Use It': { emoji: '🎯', icon: Target },
  'Live Page Interpretation': { emoji: '📊', icon: BarChart3 },
  'Map Pattern': { emoji: '🗺️', icon: MapPinned },
  'Plain English': { emoji: '💬', icon: MessageCircle },
  'Play Pattern': { emoji: '🧭', icon: Route },
  'Protector examples': { emoji: '🛡️', icon: Shield },
  'Team Needs': { emoji: '🤝', icon: Users },
  'The Five Axes': { emoji: '🧭', icon: Compass },
  'Timing Read': { emoji: '⏱️', icon: Clock3 },
  'Usually Good Into': { emoji: '✅', icon: Target },
  'Usually Struggles Into': { emoji: '🛡️', icon: Shield },
  'What It Is': { emoji: '📘', icon: BookOpen },
};

export function WinConditionsPage({ champions, onSelectChampion, onSelectCondition }: Props) {
  const orderedDefinitions = useMemo(() => orderedWinConditionDefinitions(), []);
  return (
    <section className="win-conditions-page">
      <div className="win-conditions-hero">
        <div className="win-conditions-hero-copy">
          <SectionKicker emoji="✨" icon={Sparkles} label="WinRift Strategy Model" />
          <h2>Understanding Win Conditions</h2>
          <p>
            A win condition is the practical path your team composition can use to turn champion strengths into a won game.
            WinRift reduces that messy idea into five readable strategy axes, then compares those axes against the enemy team.
          </p>
        </div>
        <div className="win-conditions-hero-icons" aria-label="Win condition shortcuts">
          {orderedDefinitions.map((definition) => (
            <button
              key={definition.key}
              onClick={() => onSelectCondition(definition.key)}
              style={{ '--condition-accent': definition.accent } as CSSProperties}
              type="button"
            >
              <img src={conditionIconUrl(definition.key)} alt="" />
              <span>{definition.label}</span>
            </button>
          ))}
        </div>
      </div>

      <div className="win-conditions-primer">
        <div className="win-conditions-primer-copy">
          <SectionKicker {...visualForHeader('Plain English')} label="Plain English" />
          <h3>What the live page is trying to tell you</h3>
          <p>
            When the live match screen says something like <strong>Team Fight B+ into Pick B</strong>, it is not saying
            "group immediately and fight no matter what." It is saying your composition has a recognizable grouped-fight identity,
            the enemy has a recognizable catch identity, and the historical sample can help frame how that kind of pairing has gone.
          </p>
          <p>
            The most useful read is usually the combination of the letter grade, the team profile bars, the game-length curve,
            and the confidence label. A 55% row with tiny evidence should feel much weaker than a 52% row with a strong sample.
          </p>
        </div>
        <div className="win-conditions-model-grid">
          {modelSteps.map((step) => {
            const Icon = step.icon;
            return (
              <article key={step.title}>
                <Icon size={19} aria-hidden="true" />
                <strong>{step.title}</strong>
                <p>{step.body}</p>
              </article>
            );
          })}
        </div>
      </div>

      <div className="win-conditions-section-heading">
        <SectionKicker {...visualForHeader('The Five Axes')} label="The Five Axes" />
        <h3>How to recognize each strategy</h3>
      </div>

      <div className="win-condition-guide-grid">
        {orderedDefinitions.map((definition) => (
          <WinConditionGuideCard
            key={definition.key}
            definition={definition}
            champions={champions}
            onSelectCondition={onSelectCondition}
            onSelectChampion={onSelectChampion}
          />
        ))}
      </div>

      <div className="win-condition-reading-card">
        <div>
          <SectionKicker {...visualForHeader('How To Use It')} label="How To Use It" />
          <h3>Read strategy as a lens, not a script</h3>
        </div>
        <ol>
          <li><strong>Start with the strongest plan.</strong> If your team has a clear primary identity, assume that is the most natural way to win.</li>
          <li><strong>Check whether the enemy is cooperating.</strong> A Pick team that keeps grouping cleanly may be easier to read as Team Fight or Control in practice.</li>
          <li><strong>Respect low-rated plans.</strong> If Split Push is C- but the row has a high winrate, treat it as correlation first. Those games probably won through another strength.</li>
          <li><strong>Use timing carefully.</strong> A good early bucket suggests urgency; a good late bucket suggests patience. Neither replaces what is happening on the map right now.</li>
        </ol>
      </div>
    </section>
  );
}

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
  const exampleChampions = definition.examples
    .map((name) => championByDisplayName(champions, name))
    .filter((champion): champion is Champion => Boolean(champion));
  const carryChampions = (definition.carryExamples ?? [])
    .map((name) => championByDisplayName(champions, name))
    .filter((champion): champion is Champion => Boolean(champion));
  const protectorChampions = (definition.protectorExamples ?? [])
    .map((name) => championByDisplayName(champions, name))
    .filter((champion): champion is Champion => Boolean(champion));
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
              <span>{definition.shortLabel}</span>
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

function WinConditionGuideCard({
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
  const exampleChampions = definition.examples
    .map((name) => championByDisplayName(champions, name))
    .filter((champion): champion is Champion => Boolean(champion));
  const carryChampions = (definition.carryExamples ?? [])
    .map((name) => championByDisplayName(champions, name))
    .filter((champion): champion is Champion => Boolean(champion));
  const protectorChampions = (definition.protectorExamples ?? [])
    .map((name) => championByDisplayName(champions, name))
    .filter((champion): champion is Champion => Boolean(champion));
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
          <span>{definition.shortLabel}</span>
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

function ExampleChampionCarousel({
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

function WinConditionCardArt({ carousel, champions }: { carousel: ChampionSplashCarouselState; champions?: ChampionData }) {
  return (
    <div className="win-condition-card-art" aria-hidden="true">
      {carousel.previous ? <WinConditionArtLayer slide={carousel.previous} champions={champions} state="exiting" /> : null}
      {carousel.active ? <WinConditionArtLayer slide={carousel.active} champions={champions} state="active" /> : null}
    </div>
  );
}

function ControlPairArt({
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

function ReadDetail({ label, text }: { label: string; text: string }) {
  return (
    <div>
      <SectionKicker {...visualForHeader(label)} label={label} />
      <p>{text}</p>
    </div>
  );
}

type HeaderVisual = {
  emoji: string;
  icon: LucideIcon;
};

function SectionKicker({ emoji, icon: Icon, label }: HeaderVisual & { label: string }) {
  return (
    <span className="win-condition-kicker">
      <em aria-hidden="true">{emoji}</em>
      <Icon size={14} strokeWidth={2.4} aria-hidden="true" />
      <strong>{label}</strong>
    </span>
  );
}

function visualForHeader(label: string): HeaderVisual {
  if (label.endsWith('champion examples')) {
    return { emoji: '⭐', icon: Sparkles };
  }
  return sectionVisuals[label] ?? { emoji: '•', icon: ListChecks };
}

function championByDisplayName(champions: ChampionData | undefined, name: string) {
  return championList(champions).find((champion) => champion.name.toLowerCase() === name.toLowerCase());
}

function orderedWinConditionDefinitions() {
  return WIN_CONDITION_PAGE_ORDER
    .map((key) => WIN_CONDITION_DEFINITIONS.find((definition) => definition.key === key))
    .filter((definition): definition is WinConditionDefinition => Boolean(definition));
}

function winConditionDefinitionByKey(key: WinConditionKey) {
  return WIN_CONDITION_DEFINITIONS.find((definition) => definition.key === key) ?? WIN_CONDITION_DEFINITIONS[0];
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

type ChampionSplashSlide = {
  champion: Champion;
  panClass: string;
  cycle: number;
};

type ChampionSplashCarouselState = {
  active?: ChampionSplashSlide;
  previous?: ChampionSplashSlide;
};

function useChampionSplashCarousel(champions: Champion[], key: string): ChampionSplashCarouselState {
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
