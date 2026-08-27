import type { AnalyticsPatchStat } from '../api/types';

export const ANALYTICS_PATCH_STORAGE_KEY = 'winrift.analyticsPatch';

export function recommendedAnalyticsPatch(options: AnalyticsPatchStat[], staticPatch: string) {
  if (!options.length) return '';
  const current = options.find((patch) => patch.patch === staticPatch);
  if (current) return current.patch;
  return [...options].sort((a, b) => comparePatchBuckets(b.patch, a.patch))[0]?.patch ?? '';
}

export function fallbackAnalyticsPatch(options: AnalyticsPatchStat[], currentPatch: string, readyMatches = 5000) {
  if (!currentPatch) return '';
  const previous = options
    .filter((option) => option.matches > 0 && comparePatchBuckets(option.patch, currentPatch) < 0)
    .sort((a, b) => comparePatchBuckets(b.patch, a.patch));
  return previous.find((option) => option.matches >= readyMatches)?.patch ?? previous[0]?.patch ?? '';
}

function comparePatchBuckets(left: string, right: string) {
  const [leftMajor = 0, leftMinor = 0] = left.split('.').map(Number);
  const [rightMajor = 0, rightMinor = 0] = right.split('.').map(Number);
  return leftMajor - rightMajor || leftMinor - rightMinor;
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
