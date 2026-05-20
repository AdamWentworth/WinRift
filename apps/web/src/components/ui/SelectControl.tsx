import type { ReactNode } from 'react';

type SelectControlProps = {
  children: ReactNode;
  className?: string;
  icon?: ReactNode;
  label?: string;
  onChange: (value: string) => void;
  value: string | number;
};

export function SelectControl({
  children,
  className = 'guide-select-control',
  icon,
  label,
  onChange,
  value,
}: SelectControlProps) {
  return (
    <label className={className}>
      {icon ?? (label ? <span>{label}</span> : null)}
      <select value={value} onChange={(event) => onChange(event.target.value)}>
        {children}
      </select>
    </label>
  );
}
