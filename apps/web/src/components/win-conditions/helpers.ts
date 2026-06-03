import type { Champion, ChampionData } from '../../api/types';
import { championList } from '../../lib/staticData';
import {
  WIN_CONDITION_DEFINITIONS,
  WIN_CONDITION_PAGE_ORDER,
  type WinConditionDefinition,
  type WinConditionKey,
} from '../../lib/winConditions';

export function championByDisplayName(champions: ChampionData | undefined, name: string) {
  return championList(champions).find((champion) => champion.name.toLowerCase() === name.toLowerCase());
}

export function orderedWinConditionDefinitions() {
  return WIN_CONDITION_PAGE_ORDER
    .map((key) => WIN_CONDITION_DEFINITIONS.find((definition) => definition.key === key))
    .filter((definition): definition is WinConditionDefinition => Boolean(definition));
}

export function winConditionDefinitionByKey(key: WinConditionKey) {
  return WIN_CONDITION_DEFINITIONS.find((definition) => definition.key === key) ?? WIN_CONDITION_DEFINITIONS[0];
}

export function matchingChampions(champions: ChampionData | undefined, names: string[]): Champion[] {
  return names
    .map((name) => championByDisplayName(champions, name))
    .filter((champion): champion is Champion => Boolean(champion));
}
