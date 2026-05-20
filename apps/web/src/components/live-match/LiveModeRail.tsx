import { Network, Swords, Users } from 'lucide-react';
import type { LiveMode } from './types';

const liveModeOptions: Array<{
  id: LiveMode;
  label: string;
  kicker: string;
  description: string;
  icon: typeof Swords;
}> = [
  {
    id: 'match',
    label: 'Match',
    kicker: 'Scout',
    description: 'Player cards and live match context',
    icon: Users,
  },
  {
    id: 'builds',
    label: 'Builds',
    kicker: 'Focused',
    description: 'Focused item path matchup stats',
    icon: Swords,
  },
  {
    id: 'winConditions',
    label: 'Win Conditions',
    kicker: 'Strategy',
    description: 'Team identity and timing',
    icon: Network,
  },
];

export function LiveModeRail({ mode, onChange }: { mode: LiveMode; onChange: (mode: LiveMode) => void }) {
  return (
    <nav className="live-mode-rail" aria-label="Live analytics mode">
      {liveModeOptions.map((option) => {
        const Icon = option.icon;
        const selected = option.id === mode;
        return (
          <button
            aria-label={`Show ${option.label} mode`}
            aria-pressed={selected}
            className={`live-mode-button${selected ? ' selected' : ''}`}
            key={option.id}
            onClick={() => onChange(option.id)}
            title={option.description}
            type="button"
          >
            <Icon aria-hidden="true" size={20} strokeWidth={2.4} />
            <span>
              <strong>{option.label}</strong>
              <em>{option.kicker}</em>
            </span>
          </button>
        );
      })}
    </nav>
  );
}
