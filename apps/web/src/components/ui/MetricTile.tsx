type MetricTileProps = {
  label: string;
  value: string | number;
  accent?: boolean;
  ariaLabel?: string;
  as?: 'div' | 'span';
  className?: string;
  labelTag?: 'span' | 'em' | 'small';
  primary?: boolean;
  tone?: string;
  valueFirst?: boolean;
  wide?: boolean;
};

export function MetricTile({
  label,
  value,
  accent = false,
  ariaLabel,
  as = 'div',
  className = 'metric-tile',
  labelTag = 'span',
  primary = false,
  tone,
  valueFirst = false,
  wide = false,
}: MetricTileProps) {
  const Tag = as;
  const LabelTag = labelTag;
  const classes = [
    className,
    tone,
    accent ? 'accent' : '',
    primary ? 'primary' : '',
    wide ? 'wide' : '',
  ].filter(Boolean).join(' ');
  const labelNode = <LabelTag>{label}</LabelTag>;
  const valueNode = <strong>{value}</strong>;

  return (
    <Tag className={classes} aria-label={ariaLabel ?? `${label}: ${value}`}>
      {valueFirst ? (
        <>
          {valueNode}
          {labelNode}
        </>
      ) : (
        <>
          {labelNode}
          {valueNode}
        </>
      )}
    </Tag>
  );
}

export function StatChip({ label, value, primary, wide }: { label: string; value: string; primary?: boolean; wide?: boolean }) {
  return (
    <MetricTile
      as="span"
      className="stat-chip"
      label={label}
      labelTag="em"
      primary={primary}
      value={value}
      valueFirst
      wide={wide}
    />
  );
}
