import type { ReactNode } from 'react';

export type SegmentedOption<T extends string = string> = {
  value: T;
  label: string;
  icon?: ReactNode;
};

export function SegmentedControl<T extends string>({
  ariaLabel,
  className = 'profile-tab-actions',
  onChange,
  options,
  value,
}: {
  ariaLabel: string;
  className?: string;
  onChange: (value: T) => void;
  options: SegmentedOption<T>[];
  value: T;
}) {
  return (
    <div className={className} aria-label={ariaLabel}>
      {options.map((option) => (
        <button
          aria-pressed={option.value === value}
          className={option.value === value ? 'selected' : ''}
          key={option.value}
          onClick={() => onChange(option.value)}
          type="button"
        >
          {option.icon}
          <span>{option.label}</span>
        </button>
      ))}
    </div>
  );
}
