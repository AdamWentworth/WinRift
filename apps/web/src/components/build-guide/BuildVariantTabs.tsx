import type { ChampionGuideBuildVariant, ChampionGuideResponse, ItemData } from '../../api/types';
import { itemImageUrl, itemName } from '../../lib/staticData';
import { itemSignatureFromSlots, type GuideItemSlot } from './GuideItemPanels';

export const RECOMMENDED_BUILD_KEY = 'recommended';

export function BuildVariantTabs({
  variants,
  recommendedGuide,
  recommendedItemSlots,
  selectedKey,
  items,
  onSelect,
}: {
  variants: ChampionGuideBuildVariant[];
  recommendedGuide?: ChampionGuideResponse;
  recommendedItemSlots: GuideItemSlot[];
  selectedKey: string;
  items?: ItemData;
  onSelect: (key: string) => void;
}) {
  if (!variants.length && !recommendedGuide) return null;
  const recommendedPath = stableExactItemPaths(recommendedGuide?.topItemPaths ?? [])[0];
  const recommendedSlotSignature = itemSignatureFromSlots(recommendedItemSlots);
  const recommendedSummary = recommendedGuide?.summary;
  const recommendedSignature = recommendedPath?.core3Signature ?? recommendedSlotSignature;
  return (
    <div className="guide-build-variant-tabs" role="tablist" aria-label="Build variants">
      <button
        key={RECOMMENDED_BUILD_KEY}
        className={selectedKey === RECOMMENDED_BUILD_KEY ? 'active' : ''}
        type="button"
        role="tab"
        aria-selected={selectedKey === RECOMMENDED_BUILD_KEY}
        onClick={() => onSelect('')}
      >
        <span>Recommended</span>
        <ItemSignatureImages signature={recommendedSignature} items={items} limit={2} />
        <em>{recommendedSummary?.games ? `${recommendedSummary.winRate.toFixed(1)}% · ${formatNumber(recommendedSummary.games)}` : 'All collected data'}</em>
      </button>
      {variants.slice(0, 5).map((variant, index) => (
        <button
          key={variant.variantKey}
          className={variant.variantKey === selectedKey ? 'active' : ''}
          type="button"
          role="tab"
          aria-selected={variant.variantKey === selectedKey}
          onClick={() => onSelect(variant.variantKey)}
        >
          <span>{buildVariantLabel(variant, index, items)}</span>
          <ItemSignatureImages signature={variant.core2Signature || variant.core3Signature} items={items} limit={2} />
          <em>{variant.winRate.toFixed(1)}% · {formatNumber(variant.games)}</em>
        </button>
      ))}
    </div>
  );
}

export function groupBuildVariantsForDisplay(variants: ChampionGuideBuildVariant[], items?: ItemData) {
  const groups = new Map<string, ChampionGuideBuildVariant>();
  const representativeGames = new Map<string, number>();
  for (const variant of variants) {
    const familyLabel = buildVariantFamilyLabel(variant, items);
    const key = familyLabel ? `label:${familyLabel.toLowerCase().replace(/[^a-z0-9]+/g, '-')}` : `core:${variant.variantKey}`;
    const existing = groups.get(key);
    if (!existing) {
      groups.set(key, { ...variant, variantKey: key, variantLabel: familyLabel || variant.variantLabel });
      representativeGames.set(key, variant.games);
      continue;
    }
    const wins = existing.wins + variant.wins;
    const games = existing.games + variant.games;
    const next: ChampionGuideBuildVariant = {
      ...existing,
      wins,
      games,
      winRate: games ? (wins / games) * 100 : 0,
      confidence: Math.max(existing.confidence, variant.confidence),
      buildCount: existing.buildCount + variant.buildCount,
      variantTags: Array.from(new Set([...(existing.variantTags ?? []), ...(variant.variantTags ?? [])])),
    };
    if (variant.games > (representativeGames.get(key) ?? 0)) {
      next.core2Signature = variant.core2Signature;
      next.core3Signature = variant.core3Signature;
      next.finalItemsSignature = variant.finalItemsSignature;
      next.runeSignature = variant.runeSignature;
      next.spellSignature = variant.spellSignature;
      next.skillOrderSignature = variant.skillOrderSignature;
      next.skillOrderWins = variant.skillOrderWins;
      next.skillOrderGames = variant.skillOrderGames;
      next.skillOrderWinRate = variant.skillOrderWinRate;
      next.skillOrderConfidence = variant.skillOrderConfidence;
      representativeGames.set(key, variant.games);
    } else if (!next.skillOrderSignature && variant.skillOrderSignature) {
      next.skillOrderSignature = variant.skillOrderSignature;
      next.skillOrderWins = variant.skillOrderWins;
      next.skillOrderGames = variant.skillOrderGames;
      next.skillOrderWinRate = variant.skillOrderWinRate;
      next.skillOrderConfidence = variant.skillOrderConfidence;
    }
    groups.set(key, next);
  }
  return Array.from(groups.values()).sort((a, b) => {
    if (a.games !== b.games) return b.games - a.games;
    return b.winRate - a.winRate;
  });
}

export function buildVariantLabel(variant: ChampionGuideBuildVariant, index: number, items?: ItemData) {
  const familyLabel = buildVariantFamilyLabel(variant, items);
  if (familyLabel) return familyLabel;
  return `Variant ${index + 1}`;
}

function ItemSignatureImages({ signature, items, limit }: { signature: string; items?: ItemData; limit?: number }) {
  const itemIds = signatureItems(signature).slice(0, limit ?? 99);
  return (
    <div className="guide-build-path-icons">
      {itemIds.length ? itemIds.map((itemId) => {
        const src = itemImageUrl(items, String(itemId));
        return src ? (
          <img key={`${signature}-${itemId}`} src={src} alt={itemName(items, String(itemId))} title={itemName(items, String(itemId))} />
        ) : (
          <span key={`${signature}-${itemId}`} className="item-pill">{itemId}</span>
        );
      }) : <em>No items</em>}
    </div>
  );
}

function stableExactItemPaths(rows: { core3Signature: string; finalItemsSignature: string }[]) {
  return rows
    .filter((row) => signatureItems(row.core3Signature).length >= 3 && signatureItems(row.finalItemsSignature).length >= 3);
}

function buildVariantFamilyLabel(variant: ChampionGuideBuildVariant, items?: ItemData) {
  if (variant.variantLabel) return variant.variantLabel;
  const names = signatureItems(`${variant.core2Signature}-${variant.core3Signature}-${variant.finalItemsSignature}`)
    .map((itemId) => itemName(items, String(itemId)).toLowerCase());
  const has = (...needles: string[]) => names.some((name) => needles.some((needle) => name.includes(needle)));
  if (has('heartsteel', "jak'sho", 'randuin', 'spirit visage', 'thornmail', 'sunfire', 'unending', 'dead man', 'kaenic', 'rookern')) {
    return 'Tank';
  }
  if (has("guinsoo", "nashor", 'ruined king', "wit'", 'terminus')) {
    return 'On Hit';
  }
  if (has('kraken', 'collector', 'infinity edge', 'serylda', "youmuu", 'eclipse', 'opportunity', 'axiom', 'edge of night', 'lord dominik')) {
    return 'AD';
  }
  if (has('rabadon', 'shadowflame', 'stormsurge', 'lich bane', 'void staff', 'zhonya', 'liandry', 'luden', 'malignance')) {
    return 'AP';
  }
  const coreNames = signatureItems(variant.core2Signature)
    .map((itemId) => itemName(items, String(itemId)))
    .filter((name) => !/^\d+$/.test(name));
  return coreNames[0] ? coreNames[0].replace(/'s\b.*$/, '') : '';
}

function signatureItems(signature: string) {
  return signature
    .split('-')
    .map((part) => Number(part))
    .filter((itemId) => itemId > 0);
}

function formatNumber(value: number) {
  return new Intl.NumberFormat('en-US').format(value);
}
