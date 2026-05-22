import { CircleSlash } from 'lucide-react';
import type { ReactNode } from 'react';

type PanelCardProps = {
  as?: 'article' | 'div' | 'section';
  children: ReactNode;
  className?: string;
};

export function PanelCard({ as = 'article', children, className = 'guide-card' }: PanelCardProps) {
  const Tag = as;

  return <Tag className={className}>{children}</Tag>;
}

export function PanelTitle({ title, detail, className = 'guide-section-title' }: { title: string; detail?: string; className?: string }) {
  return (
    <header className={className}>
      <span>{title}</span>
      {detail ? <em>{detail}</em> : null}
    </header>
  );
}

export function EmptyState({
  body,
  className = 'guide-empty',
  icon,
  message,
  title,
}: {
  body?: string;
  className?: string;
  icon?: ReactNode | false;
  message?: string;
  title?: string;
}) {
  const iconNode = icon === false ? null : icon ?? <CircleSlash size={18} />;
  return (
    <div className={className}>
      {iconNode}
      {title ? <strong>{title}</strong> : null}
      <span>{body ?? message}</span>
    </div>
  );
}
