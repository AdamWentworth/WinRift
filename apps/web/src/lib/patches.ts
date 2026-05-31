export function patchBucketFromVersion(version?: string) {
  const match = version?.match(/^(\d+\.\d+)/);
  return match?.[1] ?? '';
}
