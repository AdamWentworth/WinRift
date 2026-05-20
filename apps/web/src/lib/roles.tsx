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
      <g transform="scale(1.5)">
        <path d="M10.133 5.867H5.867v4.266h4.266z" fill="currentColor" opacity="0.42" />
        <path d="M0 0v14.75l2.828-2.829V2.828h9.094L14.75 0z" fill="currentColor" opacity="0.42" />
        <path d="M16 16V1.25L13.172 4.08v9.093H4.078L1.25 16z" fill="currentColor" />
      </g>
    );
  }
  if (role === 'UTILITY') {
    return (
      <g transform="scale(1.5)">
        <path
          d="M8.43 5.67l1.703 8.616L8 16l-2.133-1.714L7.57 5.67 8 6.249l.43-.578zM16 3.404c-.304 2.498-4.119 2.261-4.119 2.261l1.75 2.315-2.81 1.13L9.6 5.234l1.857-1.83zm-11.457 0L6.4 5.233 5.18 9.11 2.368 7.98l1.75-2.316S.305 5.901 0 3.403h4.543zM9.998 0l.669 1.185L8 4.456 5.333 1.185 6.003 0h3.995z"
          fill="currentColor"
          fillRule="evenodd"
        />
      </g>
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
