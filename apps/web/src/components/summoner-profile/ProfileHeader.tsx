import { Gamepad2, UserRound } from 'lucide-react';
import { platformLabel } from '../../lib/lookup';
import { profileIconUrl } from '../../lib/staticData';

type ProfileHeaderProps = {
  exactRiotId: string;
  platform: string;
  staticVersion?: string;
  profileIconId?: number;
  summonerLevel?: number;
  canCheckLive: boolean;
  liveAvailable: boolean;
  liveLoading: boolean;
  onShowLive: () => void;
};

export function ProfileHeader({
  exactRiotId,
  platform,
  staticVersion,
  profileIconId,
  summonerLevel,
  canCheckLive,
  liveAvailable,
  liveLoading,
  onShowLive,
}: ProfileHeaderProps) {
  const summonerIcon = profileIconUrl(staticVersion, profileIconId);
  return (
    <>
      <div className="profile-card-header">
        {summonerIcon ? (
          <img className="profile-card-icon" src={summonerIcon} alt={`${exactRiotId} profile icon`} />
        ) : (
          <UserRound size={24} />
        )}
        <div>
          <span>Summoner Profile</span>
          <strong>{exactRiotId || 'Search for a Riot ID'}</strong>
        </div>
        <em>{platformLabel(platform)}{summonerLevel ? ` · Level ${formatNumber(summonerLevel)}` : ''}</em>
      </div>

      {canCheckLive ? (
        <div className="profile-action-row">
          <button
            className={`profile-action-button ${liveAvailable ? 'live' : ''}`}
            disabled={!liveAvailable}
            onClick={onShowLive}
            type="button"
          >
            <Gamepad2 size={16} />
            <span>Live Match</span>
            <em>{liveAvailable ? 'Live now' : liveLoading ? 'Checking...' : 'Not live'}</em>
          </button>
        </div>
      ) : null}
    </>
  );
}

function formatNumber(value: number) {
  return new Intl.NumberFormat('en-US').format(value);
}
