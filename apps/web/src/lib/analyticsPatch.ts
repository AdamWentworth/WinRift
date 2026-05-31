import type { AnalyticsPatchStat } from '../api/types';

export const PATCH_READY_MATCHES = 5000;
export const ANALYTICS_PATCH_STORAGE_KEY = 'winrift.analyticsPatch';

export function recommendedAnalyticsPatch(options: AnalyticsPatchStat[], staticPatch: string) {
  if (!options.length) return '';
  const current = options.find((patch) => patch.patch === staticPatch);
  if (current && current.matches >= PATCH_READY_MATCHES) {
    return current.patch;
  }
  const bestSample = [...options].sort((a, b) => b.matches - a.matches)[0];
  return bestSample?.patch ?? current?.patch ?? options[0]?.patch ?? '';
}

export function storedAnalyticsPatch() {
  try {
    return window.localStorage.getItem(ANALYTICS_PATCH_STORAGE_KEY) ?? '';
  } catch {
    return '';
  }
}

export function storeAnalyticsPatch(patch: string) {
  try {
    window.localStorage.setItem(ANALYTICS_PATCH_STORAGE_KEY, patch);
  } catch {
    // Browser storage is optional; the selector still works for this session.
  }
}
