import type { SVGProps } from 'react';

export type RoleCode = 'TOP' | 'JUNGLE' | 'MIDDLE' | 'BOTTOM' | 'UTILITY';

export type RoleOption = {
  value: RoleCode;
  label: string;
};

export const ROLE_OPTIONS: RoleOption[] = [
  { value: 'TOP', label: 'Top' },
  { value: 'JUNGLE', label: 'Jungle' },
  { value: 'MIDDLE', label: 'Mid' },
  { value: 'BOTTOM', label: 'Bot' },
  { value: 'UTILITY', label: 'Support' },
];

export const ROLE_OPTIONS_WITH_ALL = [
  { value: '', label: 'All Roles' },
  ...ROLE_OPTIONS,
];

export function roleLabel(role?: string) {
  if (!role) return 'All Roles';
  const normalized = normalizeRole(role);
  if (!normalized && role.toUpperCase() === 'ALL') return 'All Roles';
  return ROLE_OPTIONS.find((candidate) => candidate.value === normalized)?.label ?? role;
}

export function normalizeRole(role?: string): RoleCode | '' {
  const normalized = (role ?? '').toUpperCase();
  if (normalized === 'TOP' || normalized === 'JUNGLE' || normalized === 'MIDDLE' || normalized === 'BOTTOM' || normalized === 'UTILITY') {
    return normalized;
  }
  return '';
}

type RoleIconProps = Omit<SVGProps<SVGSVGElement>, 'role'> & {
  role?: string;
  title?: string;
};

export function RoleIcon({ role: laneRole, title, className = '', ...props }: RoleIconProps) {
  const normalized = normalizeRole(laneRole);
  const label = title ?? roleLabel(normalized);
  return (
    <svg
      aria-hidden={title ? undefined : true}
      aria-label={title}
      className={`role-icon role-icon-${normalized || 'ALL'}${className ? ` ${className}` : ''}`}
      fill="none"
      role={title ? 'img' : undefined}
      viewBox="0 0 24 24"
      {...props}
    >
      {title ? <title>{label}</title> : null}
      <RoleGlyph role={normalized} />
    </svg>
  );
}

function RoleGlyph({ role }: { role: RoleCode | '' }) {
  if (role === 'TOP') {
    return (
      <>
        <path d="M5 19V5h14" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.2" />
        <path d="M8 16 19 5" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.45" opacity="0.65" />
        <path d="M5 19h6" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.2" opacity="0.55" />
      </>
    );
  }
  if (role === 'JUNGLE') {
    return (
      <g transform="scale(1.5)">
        <path
          d="M3.078 0s8.57 8.931 5.13 16c0 0-2.414-3.123-5.31-4.548 0 0 .482-4.22-2.898-7.014 0 0 4.587 1.973 5.553 5.041 0 0 1.086-4.383-2.475-9.479zM16 4s-3.44 3.068-2.837 6.85c0 0-2.414 1.424-2.836 2.3 0 0 .241-6.63 5.673-9.15zm-3.393-4s-2.656 4.603-2.414 8c0 0-.543.767-.725 1.425 0 0-.663-2.52-1.207-3.124l.016-.03c.202-.386 2.32-4.395 4.33-6.271z"
          fill="currentColor"
          fillRule="evenodd"
        />
      </g>
    );
  }
  if (role === 'MIDDLE') {
    return (
      <>
        <path d="M5 19 19 5" stroke="currentColor" strokeLinecap="round" strokeWidth="2.2" />
        <path d="m12 7.2 4.8 4.8-4.8 4.8L7.2 12 12 7.2Z" stroke="currentColor" strokeLinejoin="round" strokeWidth="1.7" opacity="0.72" />
        <path d="M5 14v5h5" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.55" opacity="0.5" />
        <path d="M14 5h5v5" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.55" opacity="0.5" />
      </>
    );
  }
  if (role === 'BOTTOM') {
    return (
      <>
        <path d="M5 5h14v14" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.2" />
        <path d="M5 19 16 8" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.45" opacity="0.65" />
        <path d="M13 19h6" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2.2" opacity="0.55" />
      </>
    );
  }
  if (role === 'UTILITY') {
    return (
      <>
        <path d="M12 4.5 16.6 7v4.4c0 3.6-1.6 6-4.6 8.1-3-2.1-4.6-4.5-4.6-8.1V7L12 4.5Z" stroke="currentColor" strokeLinejoin="round" strokeWidth="1.8" />
        <path d="M4.6 10.2c1.9.2 3.2.9 4 2.2" stroke="currentColor" strokeLinecap="round" strokeWidth="1.6" opacity="0.62" />
        <path d="M19.4 10.2c-1.9.2-3.2.9-4 2.2" stroke="currentColor" strokeLinecap="round" strokeWidth="1.6" opacity="0.62" />
        <path d="M12 8v6" stroke="currentColor" strokeLinecap="round" strokeWidth="1.6" opacity="0.75" />
      </>
    );
  }
  return (
    <>
      <circle cx="7" cy="7" fill="currentColor" r="2.1" />
      <circle cx="17" cy="7" fill="currentColor" r="2.1" opacity="0.78" />
      <circle cx="12" cy="12" fill="currentColor" r="2.1" opacity="0.9" />
      <circle cx="7" cy="17" fill="currentColor" r="2.1" opacity="0.62" />
      <circle cx="17" cy="17" fill="currentColor" r="2.1" opacity="0.62" />
    </>
  );
}
