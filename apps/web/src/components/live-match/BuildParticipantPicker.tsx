import type { ChampionData } from '../../api/types';
import { RoleIcon, roleLabel } from '../../lib/roles';
import { championByKey, championImageUrl } from '../../lib/staticData';
import type { BuildParticipantOption } from './types';
import { participantDisplayName } from './participantLabels';

export function BuildParticipantPicker({
  title,
  options,
  selectedKey,
  champions,
  onSelect,
}: {
  title: string;
  options: BuildParticipantOption[];
  selectedKey: string;
  champions?: ChampionData;
  onSelect: (key: string) => void;
}) {
  const actionLabel = title === 'Build Target' ? 'Build For' : title === 'Opponent' ? 'Against' : title;
  return (
    <div className="focused-build-picker">
      <span>{title}</span>
      <div>
        {options.map((option) => {
          const optionChampion = championByKey(champions, option.participant.championId);
          const optionUrl = championImageUrl(champions, option.participant.championId);
          const labelName = participantDisplayName(option.participant);
          const championName = optionChampion?.name ?? String(option.participant.championId);
          return (
            <button
              className={`${option.side}${option.key === selectedKey ? ' selected' : ''}`}
              key={option.key}
              onClick={() => onSelect(option.key)}
              type="button"
              aria-label={`${actionLabel} ${labelName}`}
              title={`${championName} - ${roleLabel(option.role)} - ${labelName}`}
            >
              {optionUrl ? <img src={optionUrl} alt="" /> : null}
              <small><RoleIcon role={option.role} /> {roleLabel(option.role)}</small>
            </button>
          );
        })}
      </div>
    </div>
  );
}
