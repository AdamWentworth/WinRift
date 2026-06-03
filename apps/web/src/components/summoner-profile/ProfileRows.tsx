import type { ChampionData, ChampionRecord, ItemData, RuneData, SummonerBuildRecord, SummonerRecentMatch, SummonerSpellData } from '../../api/types';
import { RoleIcon, roleLabel } from '../../lib/roles';
import {
  championByKey,
  championImageUrl,
  itemImageUrl,
  itemName,
  parseRuneSignature,
  runeImageUrl,
  runeName,
  runeStyleImageUrl,
  runeStyleName,
  signatureItems,
  signatureSpells,
  summonerSpellImageUrl,
  summonerSpellName,
} from '../../lib/staticData';
import { MiniStat } from '../ui/MetricTile';
import { championName, formatDuration, formatGameDate, formatNumber } from './profileFormatters';

export function ChampionComfortRow({ record, champions }: { record: ChampionRecord; champions?: ChampionData }) {
  const champion = championByKey(champions, record.championId);
  return (
    <div className="profile-champion-row">
      <img src={championImageUrl(champions, record.championId)} alt={champion?.name ?? String(record.championId)} />
      <div>
        <strong>{champion?.name ?? `Champion ${record.championId}`}</strong>
        <span>{record.role ? <><RoleIcon role={record.role} /> {roleLabel(record.role)} · </> : null}{record.avgKills.toFixed(1)} / {record.avgDeaths.toFixed(1)} / {record.avgAssists.toFixed(1)} average</span>
      </div>
      <div className="profile-row-stats">
        <MiniStat label="WR" value={`${record.winRate.toFixed(1)}%`} />
        <MiniStat label="KDA" value={record.kda.toFixed(2)} />
        <MiniStat label="Games" value={formatNumber(record.games)} />
      </div>
    </div>
  );
}

export function RecentMatchRow({ match, champions }: { match: SummonerRecentMatch; champions?: ChampionData }) {
  const champion = championByKey(champions, match.championId);
  const championLabel = champion?.name ?? `Champion ${match.championId}`;
  return (
    <div className={`profile-match-row ${match.win ? 'win' : 'loss'}`}>
      <img src={championImageUrl(champions, match.championId)} alt={champion?.name ?? String(match.championId)} />
      <span className={`profile-match-result-badge ${match.win ? 'win' : 'loss'}`}>{match.win ? 'Win' : 'Loss'}</span>
      <div>
        <strong>{championLabel}</strong>
        <span className="profile-match-meta"><RoleIcon role={match.role} /> {roleLabel(match.role)} · Patch {match.patch} · {formatGameDate(match.gameStartTimestamp)} · {formatDuration(match.durationSeconds)}</span>
      </div>
      <div className="profile-row-stats">
        <MiniStat label="KDA" value={`${match.kills}/${match.deaths}/${match.assists}`} />
        <MiniStat label="Role" value={roleLabel(match.role)} />
        <MiniStat label="Duration" value={formatDuration(match.durationSeconds)} />
      </div>
    </div>
  );
}

export function BuildUsageRow({
  build,
  champions,
  items,
  spells,
  runes,
}: {
  build: SummonerBuildRecord;
  champions?: ChampionData;
  items?: ItemData;
  spells?: SummonerSpellData;
  runes?: RuneData;
}) {
  const coreItems = signatureItems(build.core3Signature || build.core2Signature);
  const finalItems = signatureItems(build.finalItemsSignature);
  const displayedCore = coreItems.length ? coreItems : finalItems.slice(0, 3);
  const parsedRunes = parseRuneSignature(build.runeSignature);
  const primaryRuneStyleSrc = runeStyleImageUrl(runes, parsedRunes.primaryStyleId);
  const runeIds = parsedRunes.runeIds.slice(0, 4);
  const spellIds = signatureSpells(build.spellSignature);
  return (
    <div className="profile-build-row">
      <div className="profile-build-identity">
        <img src={championImageUrl(champions, build.championId)} alt={championName(champions, build.championId)} />
        <div>
          <strong>{championName(champions, build.championId)}</strong>
          <span><RoleIcon role={build.role} /> {roleLabel(build.role)}</span>
        </div>
      </div>
      <div className="profile-build-paths">
        <div className="profile-build-path">
          <em>Core</em>
          <ItemIconList itemIds={displayedCore} items={items} />
        </div>
        <div className="profile-build-path">
          <em>Final</em>
          <ItemIconList itemIds={finalItems} items={items} />
        </div>
      </div>
      <div className="profile-build-loadout">
        <div>
          <em>Runes</em>
          <div className="profile-build-icon-row">
            {primaryRuneStyleSrc ? (
              <img src={primaryRuneStyleSrc} alt={runeStyleName(runes, parsedRunes.primaryStyleId)} title={runeStyleName(runes, parsedRunes.primaryStyleId)} />
            ) : null}
            {runeIds.map((runeId) => {
              const src = runeImageUrl(runes, runeId);
              return src ? <img key={runeId} src={src} alt={runeName(runes, runeId)} title={runeName(runes, runeId)} /> : null;
            })}
          </div>
        </div>
        <div>
          <em>Spells</em>
          <div className="profile-build-icon-row">
            {spellIds.map((spellId) => {
              const src = summonerSpellImageUrl(spells, spellId);
              return src ? <img key={spellId} src={src} alt={summonerSpellName(spells, spellId)} title={summonerSpellName(spells, spellId)} /> : null;
            })}
          </div>
        </div>
      </div>
      <div className="profile-row-stats">
        <MiniStat label="Games" value={formatNumber(build.games)} />
        <MiniStat label="WR" value={`${build.winRate.toFixed(1)}%`} />
        <MiniStat label="KDA" value={build.kda.toFixed(2)} />
      </div>
    </div>
  );
}

function ItemIconList({ itemIds, items }: { itemIds: string[]; items?: ItemData }) {
  if (!itemIds.length) {
    return <span className="profile-build-empty-path">No item path</span>;
  }
  return (
    <div className="profile-build-icon-row">
      {itemIds.slice(0, 6).map((itemId, index) => {
        const src = itemImageUrl(items, itemId);
        return src ? (
          <img key={`${itemId}:${index}`} src={src} alt={itemName(items, itemId)} title={itemName(items, itemId)} />
        ) : (
          <span key={`${itemId}:${index}`} className="profile-build-item-fallback">{itemId}</span>
        );
      })}
    </div>
  );
}
