import { RadioTower, Search } from 'lucide-react';
import { useState } from 'react';
import type { BuildFilters, ChampionData, LiveGame, LiveParticipant } from '../api/types';
import { championByKey, championImageUrl } from '../lib/staticData';

type Props = {
  champions?: ChampionData;
  liveGame?: LiveGame;
  loading: boolean;
  error?: Error | null;
  onSearch: (gameName: string, tagLine: string, platform: string) => void;
  onUseContext: (filters: BuildFilters) => void;
};

export function LiveMatchPanel({ champions, liveGame, loading, error, onSearch, onUseContext }: Props) {
  const [gameName, setGameName] = useState('');
  const [tagLine, setTagLine] = useState('');
  const [platform, setPlatform] = useState('NA1');
  const [focus, setFocus] = useState<LiveParticipant | null>(null);
  const [opponent, setOpponent] = useState<LiveParticipant | null>(null);
  const [role, setRole] = useState('TOP');

  const applyContext = () => {
    if (!focus || !opponent) return;
    onUseContext({
      championId: focus.championId,
      opponentChampionId: opponent.championId,
      role,
      minGames: 5,
    });
  };

  return (
    <section className="panel">
      <div className="panel-heading">
        <h2>Live Context</h2>
        <RadioTower size={18} />
      </div>
      <div className="live-form">
        <input value={gameName} placeholder="Game Name" onChange={(event) => setGameName(event.target.value)} />
        <input value={tagLine} placeholder="Tag" onChange={(event) => setTagLine(event.target.value)} />
        <select value={platform} onChange={(event) => setPlatform(event.target.value)}>
          <option value="NA1">NA</option>
          <option value="EUW1">EUW</option>
          <option value="EUN1">EUNE</option>
          <option value="KR">KR</option>
          <option value="BR1">BR</option>
          <option value="LA1">LAN</option>
          <option value="LA2">LAS</option>
          <option value="JP1">JP</option>
          <option value="OC1">OCE</option>
        </select>
        <button className="icon-button primary" onClick={() => onSearch(gameName, tagLine, platform)} title="Find live game" aria-label="Find live game">
          <Search size={18} />
        </button>
      </div>
      {loading && <div className="state-panel compact-state">Checking live game...</div>}
      {error && <div className="state-panel compact-state">{error.message}</div>}
      {liveGame && (
        <>
          <div className="role-row">
            <label>
              Context role
              <select value={role} onChange={(event) => setRole(event.target.value)}>
                {['TOP', 'JUNGLE', 'MIDDLE', 'BOTTOM', 'UTILITY'].map((value) => (
                  <option key={value} value={value}>{value}</option>
                ))}
              </select>
            </label>
            <button className="text-button" onClick={applyContext} disabled={!focus || !opponent}>
              Use Context
            </button>
          </div>
          <div className="teams-grid">
            {[100, 200].map((teamId) => (
              <div className="team-list" key={teamId}>
                <h3>{teamId === 100 ? 'Blue' : 'Red'}</h3>
                {liveGame.participants.filter((participant) => participant.teamId === teamId).map((participant) => {
                  const champion = championByKey(champions, participant.championId);
                  const selected = focus === participant || opponent === participant;
                  return (
                    <button
                      className={selected ? 'live-player selected' : 'live-player'}
                      key={`${participant.teamId}-${participant.championId}-${participant.summonerId}`}
                      onClick={() => (focus?.teamId === participant.teamId || !focus ? setFocus(participant) : setOpponent(participant))}
                    >
                      <img src={championImageUrl(champions, participant.championId)} alt={champion?.name ?? String(participant.championId)} />
                      <span>{champion?.name ?? participant.championId}</span>
                    </button>
                  );
                })}
              </div>
            ))}
          </div>
        </>
      )}
    </section>
  );
}
