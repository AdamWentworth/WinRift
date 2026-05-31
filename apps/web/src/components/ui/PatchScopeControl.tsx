import type { AnalyticsPatchStat } from '../../api/types';

const PATCH_READY_MATCHES = 5000;

type PatchScopeControlProps = {
  activePatch: string;
  className?: string;
  currentPatch: string;
  loading: boolean;
  options: AnalyticsPatchStat[];
  onChange: (patch: string) => void;
};

export function PatchScopeControl({
  activePatch,
  className = '',
  currentPatch,
  loading,
  options,
  onChange,
}: PatchScopeControlProps) {
  const visibleOptions = options.length
    ? options
    : activePatch
      ? [{ patch: activePatch, matches: 0, participantSamples: 0, rawMatches: 0, compiledMatches: 0, current: activePatch === currentPatch }]
      : [];
  const activeOption = visibleOptions.find((option) => option.patch === activePatch);
  const activeMatches = activeOption?.matches ?? 0;
  const isCurrentPatch = activePatch === currentPatch;
  const statusLabel = patchStatusLabel(loading, isCurrentPatch, activeMatches);
  const detailLabel = activeMatches ? `${formatNumber(activeMatches)} matches indexed` : loading ? 'Loading indexed sample' : 'Sample pending';

  if (!visibleOptions.length) {
    return (
      <div className={`patch-scope-control loading ${className}`.trim()} aria-label="Analytics patch loading">
        <span>Data Patch</span>
        <b>{loading ? 'Loading' : 'Current'}</b>
        <em>{loading ? 'Checking stored patches' : 'No patch samples yet'}</em>
      </div>
    );
  }

  return (
    <label className={`patch-scope-control ${className}`.trim()}>
      <span>Data Patch</span>
      <select aria-label="Analytics data patch" value={activePatch} onChange={(event) => onChange(event.target.value)}>
        {visibleOptions.map((option) => (
          <option key={option.patch} value={option.patch}>
            {option.patch}{option.patch === currentPatch ? ' current' : ''} · {formatNumber(option.matches)} matches
          </option>
        ))}
      </select>
      <em>{statusLabel} · {detailLabel}</em>
    </label>
  );
}

function patchStatusLabel(loading: boolean, isCurrentPatch: boolean, matches: number) {
  if (loading) return 'Loading';
  if (!isCurrentPatch) return 'Previous patch selected';
  if (matches > 0 && matches < PATCH_READY_MATCHES) return 'Current patch still filling';
  return 'Current patch';
}

function formatNumber(value: number) {
  return new Intl.NumberFormat('en-US').format(value);
}
