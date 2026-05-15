import type { AnalyticsBuild, ChampionData, ItemData } from '../api/types';
import { championByKey, itemImageUrl, itemName, signatureItems } from '../lib/staticData';

type Props = {
  builds: AnalyticsBuild[];
  champions?: ChampionData;
  items?: ItemData;
  loading: boolean;
};

export function BuildResults({ builds, champions, items, loading }: Props) {
  if (loading) {
    return <div className="state-panel">Loading build patterns...</div>;
  }

  if (!builds.length) {
    return <div className="state-panel">No build patterns match the current filters.</div>;
  }

  return (
    <div className="results-table" aria-label="Build matchup results">
      <div className="results-row results-head">
        <span>Champion</span>
        <span>Opponent</span>
        <span>Core</span>
        <span>Final Items</span>
        <span>Spells</span>
        <span>Winrate</span>
        <span>Confidence</span>
        <span>Games</span>
      </div>
      {builds.map((build) => {
        const champion = championByKey(champions, build.championId);
        const opponent = championByKey(champions, build.opponentChampionId);
        return (
          <div className="results-row" key={`${build.championId}-${build.opponentChampionId}-${build.finalItemsSignature}-${build.runeSignature}`}>
            <span>{champion?.name ?? build.championId}</span>
            <span>{opponent?.name ?? build.opponentChampionId}</span>
            <ItemSignature signature={build.core3Signature || build.core2Signature} items={items} compact />
            <ItemSignature signature={build.finalItemsSignature} items={items} />
            <span>{build.spellSignature}</span>
            <strong>{build.winRate.toFixed(1)}%</strong>
            <span>{build.confidence.toFixed(1)}%</span>
            <span className={build.games < 20 ? 'sample low-sample' : 'sample'}>{build.games}</span>
          </div>
        );
      })}
    </div>
  );
}

function ItemSignature({ signature, items, compact = false }: { signature: string; items?: ItemData; compact?: boolean }) {
  const ids = signatureItems(signature);
  if (!ids.length) return <span className="muted">None</span>;

  return (
    <span className={compact ? 'item-strip compact' : 'item-strip'}>
      {ids.map((itemId) => {
        const imageUrl = itemImageUrl(items, itemId);
        return imageUrl ? (
          <img key={`${signature}-${itemId}`} src={imageUrl} alt={itemName(items, itemId)} title={itemName(items, itemId)} />
        ) : (
          <span key={`${signature}-${itemId}`} className="item-pill">{itemId}</span>
        );
      })}
    </span>
  );
}
