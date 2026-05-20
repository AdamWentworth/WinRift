import type { ReactNode } from 'react';

type StatusChipProps = {
  children: ReactNode;
  ariaLabel?: string;
  as?: 'span' | 'b' | 'small';
  className: string;
  title?: string;
  tone?: string;
};

export function StatusChip({ children, ariaLabel, as = 'span', className, title, tone }: StatusChipProps) {
  const Tag = as;
  const classes = [className, tone].filter(Boolean).join(' ');

  return (
    <Tag aria-label={ariaLabel} className={classes} title={title}>
      {children}
    </Tag>
  );
}
