import type { ReactNode } from 'react';
import type { Champion, ChampionData } from '../../api/types';
import { championImageUrl } from '../../lib/staticData';

type ChampionIdentityProps = {
  as?: 'div' | 'span';
  champion?: Champion;
  championId: number;
  champions?: ChampionData;
  className: string;
  detail?: ReactNode;
};

export function ChampionIdentity({
  as = 'span',
  champion,
  championId,
  champions,
  className,
  detail,
}: ChampionIdentityProps) {
  const Tag = as;

  return (
    <Tag className={className}>
      {champion ? <img src={championImageUrl(champions, championId)} alt="" /> : null}
      <span>
        <strong>{champion?.name ?? championId}</strong>
        {detail ? <em>{detail}</em> : null}
      </span>
    </Tag>
  );
}
