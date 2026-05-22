import type { ChampionData, LiveParticipant, WinConditionAnalysisResponse } from '../../api/types';
import { RoleIcon, roleLabel } from '../../lib/roles';
import { championByKey, championImageUrl } from '../../lib/staticData';
import { WinConditionPanel } from './WinConditionPanel';
import { roles, type TeamSide } from './types';
import { liveParticipantName, participantKey } from './utils';

export function WinConditionFocusedView({
  blueTeam,
  redTeam,
  yourSide,
  champions,
  analysis,
  loading,
  error,
  ready,
}: {
  blueTeam: LiveParticipant[];
  redTeam: LiveParticipant[];
  yourSide: TeamSide;
  champions?: ChampionData;
  analysis?: WinConditionAnalysisResponse;
  loading: boolean;
  error?: string;
  ready: boolean;
}) {
  const enemySide: TeamSide = yourSide === 'blue' ? 'red' : 'blue';
  const yourTeam = yourSide === 'blue' ? blueTeam : redTeam;
  const enemyTeam = enemySide === 'blue' ? blueTeam : redTeam;

  return (
    <section className="win-condition-focused" aria-label="Focused win condition view">
      <WinConditionTeamStrip
        side={yourSide}
        title="Your Team"
        team={yourTeam}
        champions={champions}
      />
      <div className="win-condition-focus-main">
        {ready ? (
          <WinConditionPanel
            analysis={analysis}
            yourSide={yourSide}
            loading={loading}
            error={error}
          />
        ) : (
          <section className="win-condition-panel win-condition-state">
            Win condition metrics need five ordered champions on each side. Switch to Match mode to correct lane order if Riot's live data looks wrong.
          </section>
        )}
      </div>
      <WinConditionTeamStrip
        side={enemySide}
        title="Enemy Team"
        team={enemyTeam}
        champions={champions}
      />
    </section>
  );
}

function WinConditionTeamStrip({
  side,
  title,
  team,
  champions,
}: {
  side: TeamSide;
  title: string;
  team: LiveParticipant[];
  champions?: ChampionData;
}) {
  return (
    <div className={`win-team-strip ${side}`}>
      <header>
        <strong>{title}</strong>
        <span>{side === 'blue' ? 'Blue side' : 'Red side'}</span>
      </header>
      <div className="win-team-slots">
        {roles.map((role, index) => (
          <WinConditionTeamSlot
            key={team[index] ? participantKey(team[index], index) : `${side}-${role}`}
            participant={team[index]}
            role={role}
            champions={champions}
          />
        ))}
      </div>
    </div>
  );
}

function WinConditionTeamSlot({
  participant,
  role,
  champions,
}: {
  participant?: LiveParticipant;
  role: string;
  champions?: ChampionData;
}) {
  const champion = participant ? championByKey(champions, participant.championId) : undefined;
  const imageUrl = participant ? championImageUrl(champions, participant.championId) : '';
  const championName = champion?.name ?? (participant ? `Champion ${participant.championId}` : 'Open slot');

  return (
    <div className={`win-team-slot${participant ? '' : ' empty'}`}>
      <span className="win-team-avatar">
        {imageUrl ? <img src={imageUrl} alt="" /> : <em>{championName.charAt(0)}</em>}
      </span>
      <span className="win-team-slot-copy">
        <span className="win-team-role">
          <RoleIcon role={role} />
          {roleLabel(role)}
        </span>
        <strong>{championName}</strong>
        <em>{participant ? liveParticipantName(participant) : 'Missing player'}</em>
      </span>
    </div>
  );
}
