import {
  AlertTriangle,
  BarChart3,
  BookOpen,
  Clock3,
  Compass,
  Eye,
  ListChecks,
  MapPinned,
  MessageCircle,
  Network,
  Route,
  Shield,
  ShieldAlert,
  Sparkles,
  Swords,
  Target,
  Users,
  type LucideIcon,
} from 'lucide-react';

type HeaderVisual = {
  icon: LucideIcon;
};

const sectionVisuals: Record<string, HeaderVisual> = {
  'Carry examples': { icon: Target },
  'Common Failure': { icon: AlertTriangle },
  'Composition Signals': { icon: Network },
  'Example champions': { icon: Sparkles },
  'Failure Checks': { icon: ShieldAlert },
  'How It Wins': { icon: Swords },
  'How To Read It': { icon: Eye },
  'How To Use It': { icon: Target },
  'Live Page Interpretation': { icon: BarChart3 },
  'Map Pattern': { icon: MapPinned },
  'Plain English': { icon: MessageCircle },
  'Play Pattern': { icon: Route },
  'Protector examples': { icon: Shield },
  'Team Needs': { icon: Users },
  'The Five Axes': { icon: Compass },
  'Timing Read': { icon: Clock3 },
  'Usually Good Into': { icon: Target },
  'Usually Struggles Into': { icon: Shield },
  'What It Is': { icon: BookOpen },
  'WinRift Strategy Model': { icon: Sparkles },
};

export function SectionKicker({ icon: Icon, label }: HeaderVisual & { label: string }) {
  return (
    <span className="win-condition-kicker">
      <span className="win-condition-kicker-icon" aria-hidden="true">
        <Icon size={14} strokeWidth={2.4} />
      </span>
      <strong>{label}</strong>
    </span>
  );
}

export function visualForHeader(label: string): HeaderVisual {
  if (label.endsWith('champion examples')) {
    return { icon: Sparkles };
  }
  return sectionVisuals[label] ?? { icon: ListChecks };
}
