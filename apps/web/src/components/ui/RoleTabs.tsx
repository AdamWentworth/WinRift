import { RoleIcon } from '../../lib/roles';

type RoleTabOption = {
  value: string;
  label: string;
};

type RoleTabsProps = {
  options: RoleTabOption[];
  value: string;
  onChange: (value: string) => void;
  ariaLabel?: string;
  className?: string;
};

export function RoleTabs({ options, value, onChange, ariaLabel = 'Role', className = 'guide-role-tabs' }: RoleTabsProps) {
  return (
    <div className={className} aria-label={ariaLabel}>
      {options.map((candidate) => (
        <button
          aria-pressed={candidate.value === value}
          key={candidate.value || 'ALL'}
          className={candidate.value === value ? 'selected' : ''}
          onClick={() => onChange(candidate.value)}
          type="button"
        >
          <RoleIcon role={candidate.value} />
          <span>{candidate.label}</span>
        </button>
      ))}
    </div>
  );
}
