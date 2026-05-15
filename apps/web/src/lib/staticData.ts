import type { Champion, ChampionData, ItemData } from '../api/types';

export function championList(champions?: ChampionData): Champion[] {
  if (!champions) return [];
  return Object.values(champions.data.data).sort((a, b) => a.name.localeCompare(b.name));
}

export function championByKey(champions: ChampionData | undefined, championId: number) {
  if (!champions) return undefined;
  return Object.values(champions.data.data).find((champion) => Number(champion.key) === championId);
}

export function championImageUrl(champions: ChampionData | undefined, championId: number) {
  const champion = championByKey(champions, championId);
  if (!champion || !champions) return '';
  return `https://ddragon.leagueoflegends.com/cdn/${champions.version}/img/champion/${champion.image.full}`;
}

export function itemImageUrl(items: ItemData | undefined, itemId: string) {
  const item = items?.data.data[itemId];
  if (!item?.image || !items) return '';
  return `https://ddragon.leagueoflegends.com/cdn/${items.version}/img/item/${item.image.full}`;
}

export function itemName(items: ItemData | undefined, itemId: string) {
  return items?.data.data[itemId]?.name ?? itemId;
}

export function signatureItems(signature: string) {
  return signature ? signature.split('-').filter(Boolean) : [];
}
