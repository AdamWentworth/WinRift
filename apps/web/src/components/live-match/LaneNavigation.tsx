import { RoleIcon, roleLabel } from '../../lib/roles';
import { roles } from './types';

export function LaneHeader() {
  return (
    <div className="lane-header-row" aria-label="Matchup lanes">
      {roles.map((role) => (
        <div className="lane-header-cell" key={role}>
          <RoleIcon role={role} />
          <span>{roleLabel(role)}</span>
        </div>
      ))}
    </div>
  );
}

export function LaneTabs({ selectedIndex, onSelect }: { selectedIndex: number; onSelect: (index: number) => void }) {
  return (
    <div className="mobile-lane-tabs" aria-label="Mobile lane selector">
      {roles.map((role, index) => (
        <button
          className={index === selectedIndex ? 'selected' : ''}
          key={role}
          onClick={() => onSelect(index)}
          type="button"
        >
          <RoleIcon role={role} />
          <span>{roleLabel(role)}</span>
        </button>
      ))}
    </div>
  );
}
