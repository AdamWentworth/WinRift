import { CircleSlash } from 'lucide-react';

export function PanelTitle({ title, detail, className = 'guide-section-title' }: { title: string; detail?: string; className?: string }) {
  return (
    <header className={className}>
      <span>{title}</span>
      {detail ? <em>{detail}</em> : null}
    </header>
  );
}

export function EmptyState({ message, className = 'guide-empty' }: { message: string; className?: string }) {
  return (
    <div className={className}>
      <CircleSlash size={18} />
      <span>{message}</span>
    </div>
  );
}
