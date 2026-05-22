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
  valueTag?: 'strong' | 'b';
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
  valueTag = 'strong',
  wide = false,
}: MetricTileProps) {
  const Tag = as;
  const LabelTag = labelTag;
  const ValueTag = valueTag;
  const classes = [
    className,
    tone,
    accent ? 'accent' : '',
    primary ? 'primary' : '',
    wide ? 'wide' : '',
  ].filter(Boolean).join(' ');
  const labelNode = <LabelTag>{label}</LabelTag>;
  const valueNode = <ValueTag>{value}</ValueTag>;

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

export function MiniStat({ label, tone, value }: { label: string; tone?: string; value: string }) {
  return (
    <MetricTile
      as="span"
      className="profile-mini-stat"
      label={label}
      labelTag="em"
      tone={tone}
      value={value}
      valueTag="b"
    />
  );
}
