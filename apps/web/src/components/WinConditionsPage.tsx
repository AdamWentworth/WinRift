import { ArrowRight, BarChart3, BookOpen, MousePointer2, ShieldAlert } from 'lucide-react';
import type { CSSProperties } from 'react';
import type { Champion, ChampionData } from '../api/types';
import { championByKey, championImageUrl, championList, championSplashUrl } from '../lib/staticData';
import { conditionIconUrl, WIN_CONDITION_DEFINITIONS, type WinConditionDefinition } from '../lib/winConditions';

type Props = {
  champions?: ChampionData;
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

export function WinConditionsPage({ champions, onSelectChampion }: Props) {
  return (
    <section className="win-conditions-page">
      <div className="win-conditions-hero">
        <div className="win-conditions-hero-copy">
          <span>WinRift Strategy Model</span>
          <h2>Understanding Win Conditions</h2>
          <p>
            A win condition is the practical path your team composition can use to turn champion strengths into a won game.
            WinRift reduces that messy idea into five readable strategy axes, then compares those axes against the enemy team.
          </p>
        </div>
        <div className="win-conditions-hero-icons" aria-label="Win condition shortcuts">
          {WIN_CONDITION_DEFINITIONS.map((definition) => (
            <a
              href={`#${definition.key}`}
              key={definition.key}
              style={{ '--condition-accent': definition.accent } as CSSProperties}
            >
              <img src={conditionIconUrl(definition.key)} alt="" />
              <span>{definition.label}</span>
            </a>
          ))}
        </div>
      </div>

      <div className="win-conditions-primer">
        <div className="win-conditions-primer-copy">
          <span>Plain English</span>
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
        <span>The Five Axes</span>
        <h3>How to recognize each strategy</h3>
      </div>

      <div className="win-condition-guide-grid">
        {WIN_CONDITION_DEFINITIONS.map((definition) => (
          <WinConditionGuideCard
            key={definition.key}
            definition={definition}
            champions={champions}
            onSelectChampion={onSelectChampion}
          />
        ))}
      </div>

      <div className="win-condition-reading-card">
        <div>
          <span>How To Use It</span>
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

function WinConditionGuideCard({ definition, champions, onSelectChampion }: { definition: WinConditionDefinition; champions?: ChampionData; onSelectChampion: (champion: Champion) => void }) {
  const exampleChampions = definition.examples
    .map((name) => championByDisplayName(champions, name))
    .filter((champion): champion is Champion => Boolean(champion));
  const heroChampion = exampleChampions[0] ? championByKey(champions, Number(exampleChampions[0].key)) : undefined;
  return (
    <article
      className={`win-condition-guide-card${definition.key === 'Control' ? ' spotlight' : ''}`}
      id={definition.key}
      style={{
        '--condition-accent': definition.accent,
        '--condition-splash': heroChampion ? `url(${championSplashUrl(champions, Number(heroChampion.key))})` : 'none',
      } as CSSProperties}
    >
      <header>
        <img src={conditionIconUrl(definition.key)} alt="" />
        <div>
          <span>{definition.shortLabel}</span>
          <h3>{definition.label}</h3>
        </div>
      </header>
      <p className="win-condition-main-copy">{definition.plainEnglish}</p>
      <div className="win-condition-detail-grid">
        <ReadDetail label="Map Pattern" text={definition.mapPattern} />
        <ReadDetail label="Team Needs" text={definition.teamNeeds} />
        <ReadDetail label="Common Failure" text={definition.commonFailure} />
      </div>
      {definition.key === 'Control' ? (
        <div className="win-condition-pairing-callout">
          <span>Why Carry Pairings Matter</span>
          <p>
            Control is not just a defensive label. Janna, Braum, Poppy, Ivern, traps, walls, and slows are valuable because
            they buy time and space for a damage dealer. Pair that shell with Vayne, Kai'Sa, Kog'Maw, Jinx, or another
            protected carry and the strategy becomes much more real: the enemy has to walk through the controlled zone
            while your carry keeps dealing damage.
          </p>
        </div>
      ) : null}
      <div className="win-condition-example-strip">
        <span>Example champions</span>
        <div>
          {definition.examples.map((name) => {
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
    </article>
  );
}

function ReadDetail({ label, text }: { label: string; text: string }) {
  return (
    <div>
      <span>{label}</span>
      <p>{text}</p>
    </div>
  );
}

function championByDisplayName(champions: ChampionData | undefined, name: string) {
  return championList(champions).find((champion) => champion.name.toLowerCase() === name.toLowerCase());
}
