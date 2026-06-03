import { ArrowRight, BarChart3, BookOpen, MousePointer2, ShieldAlert, Shuffle } from 'lucide-react';
import { useMemo } from 'react';
import type { CSSProperties } from 'react';
import type { Champion, ChampionData } from '../../api/types';
import { conditionIconUrl, FLEX_ARCHETYPE_DEFINITION, type WinConditionKey } from '../../lib/winConditions';
import { ExampleChampionCarousel } from './ExampleChampionCarousel';
import { orderedWinConditionDefinitions } from './helpers';
import { SectionKicker, visualForHeader } from './SectionKicker';
import { WinConditionGuideCard } from './WinConditionGuideCard';

type Props = {
  champions?: ChampionData;
  onSelectFlex: () => void;
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

export function WinConditionsPage({ champions, onSelectChampion, onSelectCondition, onSelectFlex }: Props) {
  const orderedDefinitions = useMemo(() => orderedWinConditionDefinitions(), []);
  const flexDefinition = FLEX_ARCHETYPE_DEFINITION;
  return (
    <section className="win-conditions-page">
      <div className="win-conditions-hero">
        <div className="win-conditions-hero-copy">
          <SectionKicker {...visualForHeader('WinRift Strategy Model')} label="WinRift Strategy Model" />
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
          <button
            className="flex-shortcut"
            onClick={onSelectFlex}
            style={{ '--condition-accent': flexDefinition.accent } as CSSProperties}
            type="button"
          >
            <img src={conditionIconUrl(flexDefinition.key)} alt="" />
            <span>{flexDefinition.label}</span>
          </button>
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

      <article
        className="win-condition-flex-card"
        style={{ '--condition-accent': flexDefinition.accent } as CSSProperties}
      >
        <div className="win-condition-flex-copy">
          <SectionKicker icon={Shuffle} label="Special Archetype" />
          <header>
            <img src={conditionIconUrl(flexDefinition.key)} alt="" />
            <div>
              <span className="win-condition-card-subtitle">{flexDefinition.shortLabel}</span>
              <h3>{flexDefinition.label}</h3>
            </div>
          </header>
          <p className="win-condition-main-copy">{flexDefinition.plainEnglish}</p>
          <p>
            Think of Flex as draft glue. It is useful because it can connect your main plan to a secondary plan, but it
            still needs the team to commit to a real pressure point once the game state becomes clear.
          </p>
          <button className="win-condition-guide-link" onClick={onSelectFlex} type="button">
            Open Flex guide
            <ArrowRight size={14} aria-hidden="true" />
          </button>
        </div>
        <ExampleChampionCarousel
          championNames={flexDefinition.examples}
          champions={champions}
          conditionLabel={flexDefinition.label}
          label="Flex champion examples"
          onSelectChampion={onSelectChampion}
        />
      </article>

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
