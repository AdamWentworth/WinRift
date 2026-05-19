import type { Champion, ChampionData, ItemData, RankedRecord, Rune, RuneData, RuneStyle, SummonerSpellData } from '../api/types';

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

export function championSplashUrl(champions: ChampionData | undefined, championId: number) {
  const champion = championByKey(champions, championId);
  if (!champion) return '';
  return `https://ddragon.leagueoflegends.com/cdn/img/champion/splash/${champion.id}_0.jpg`;
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

export function signatureSpells(signature: string) {
  return signature ? signature.split('-').filter(Boolean) : [];
}

export function rankIconUrl(rank?: RankedRecord) {
  const tier = rank?.tier?.trim().toLowerCase() || 'unranked';
  return `/images/ranked_icons/${tier}.png`;
}

export function rankLabel(rank?: RankedRecord) {
  const tier = rank?.tier?.trim().toUpperCase();
  if (!tier || tier === 'UNRANKED') return 'UNRANKED';
  return `${tier} ${rank?.division?.trim().toUpperCase() ?? ''}`.trim();
}

export function summonerSpellByKey(spells: SummonerSpellData | undefined, spellId: string | number) {
  if (!spells) return undefined;
  return Object.values(spells.data.data).find((spell) => Number(spell.key) === Number(spellId));
}

export function summonerSpellImageUrl(spells: SummonerSpellData | undefined, spellId: string | number) {
  const spell = summonerSpellByKey(spells, spellId);
  if (!spell?.image || !spells) return '';
  return `https://ddragon.leagueoflegends.com/cdn/${spells.version}/img/spell/${spell.image.full}`;
}

export function summonerSpellName(spells: SummonerSpellData | undefined, spellId: string | number) {
  return summonerSpellByKey(spells, spellId)?.name ?? String(spellId);
}

export type ParsedRuneSignature = {
  primaryStyleId?: number;
  secondaryStyleId?: number;
  runeIds: number[];
  statPerks: number[];
};

export function parseRuneSignature(signature: string): ParsedRuneSignature {
  const [primaryStyle, secondaryStyle, runes, statPerks] = signature.split('|');
  return {
    primaryStyleId: positiveNumber(primaryStyle),
    secondaryStyleId: positiveNumber(secondaryStyle),
    runeIds: splitNumbers(runes),
    statPerks: splitNumbers(statPerks),
  };
}

export function runeStyleById(runes: RuneData | undefined, styleId: number | undefined): RuneStyle | undefined {
  if (!runes || !styleId) return undefined;
  return runes.data.find((style) => style.id === styleId);
}

export function runeById(runes: RuneData | undefined, runeId: number | undefined): Rune | undefined {
  if (!runes || !runeId) return undefined;
  for (const style of runes.data) {
    for (const slot of style.slots) {
      const rune = slot.runes.find((candidate) => candidate.id === runeId);
      if (rune) return rune;
    }
  }
  return undefined;
}

export function runeImageUrl(runes: RuneData | undefined, runeId: number | undefined) {
  const rune = runeById(runes, runeId);
  if (!rune?.icon) return '';
  return `https://ddragon.leagueoflegends.com/cdn/img/${rune.icon}`;
}

export function runeStyleImageUrl(runes: RuneData | undefined, styleId: number | undefined) {
  const style = runeStyleById(runes, styleId);
  if (!style?.icon) return '';
  return `https://ddragon.leagueoflegends.com/cdn/img/${style.icon}`;
}

export function runeName(runes: RuneData | undefined, runeId: number | undefined) {
  return runeById(runes, runeId)?.name ?? (runeId ? String(runeId) : 'Unknown');
}

export function runeStyleName(runes: RuneData | undefined, styleId: number | undefined) {
  return runeStyleById(runes, styleId)?.name ?? (styleId ? String(styleId) : 'Unknown');
}

function splitNumbers(value?: string) {
  if (!value) return [];
  return value
    .split('-')
    .map((part) => Number(part))
    .filter((part) => Number.isFinite(part) && part > 0);
}

function positiveNumber(value?: string) {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : undefined;
}
