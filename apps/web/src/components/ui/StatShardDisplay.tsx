import { STAT_PERK_ROWS, statPerkImageUrl, statPerkLabel } from '../../lib/staticData';

type Props = {
  selectedIds: number[];
  className?: string;
};

export function StatShardGrid({ selectedIds, className = '' }: Props) {
  const classes = ['stat-shard-grid', className].filter(Boolean).join(' ');
  return (
    <div className={classes}>
      <span className="stat-shard-grid-title">Stat Shards</span>
      {STAT_PERK_ROWS.map((row, rowIndex) => {
        const selectedId = selectedIds[rowIndex];
        return (
          <div className="stat-shard-row" key={row.key}>
            <em>{row.label}</em>
            <div>
              {row.options.map((option) => (
                <StatShardIcon key={`${row.key}-${option}`} id={option} selected={option === selectedId} />
              ))}
            </div>
          </div>
        );
      })}
    </div>
  );
}

export function StatShardIconStrip({ selectedIds, className = '' }: Props) {
  const ids = selectedIds.slice(0, 3).filter(Boolean);
  if (!ids.length) return null;
  const classes = ['stat-shard-icon-strip', className].filter(Boolean).join(' ');
  return (
    <span className={classes} aria-label="Stat shards">
      <b>Shards</b>
      {ids.map((id, index) => (
        <StatShardIcon key={`${id}-${index}`} id={id} selected />
      ))}
    </span>
  );
}

function StatShardIcon({ id, selected }: { id: number; selected?: boolean }) {
  const label = statPerkLabel(id);
  const src = statPerkImageUrl(id);
  return (
    <span className={selected ? 'stat-shard-icon selected' : 'stat-shard-icon'} title={selected ? `${label} selected` : label}>
      {src ? <img src={src} alt={label} /> : <strong>{id}</strong>}
    </span>
  );
}
