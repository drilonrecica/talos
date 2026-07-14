import type { LiveSnapshot } from './live.svelte';

const rank: Record<string, number> = {
  down: 5,
  degraded: 4,
  unknown: 3,
  paused: 2,
  healthy: 1,
};

export function prioritizedResources(
  resources: LiveSnapshot['resources'],
  pinned: string[] = [],
) {
  const available = new Map(
    resources.map((resource) => [resource.id, resource]),
  );
  const pinnedResources = pinned.flatMap((id) => {
    const resource = available.get(id);
    if (!resource) return [];
    available.delete(id);
    return [resource];
  });
  return pinnedResources.concat(
    [...available.values()].sort(
      (left, right) =>
        (rank[right.status] ?? 0) - (rank[left.status] ?? 0) ||
        left.name.localeCompare(right.name),
    ),
  );
}

export function staleResource(
  resource: LiveSnapshot['resources'][number],
  snapshotAt: string,
  thresholdSeconds = 10,
) {
  if (!resource.lastSeenAt) return true;
  return (
    new Date(snapshotAt).getTime() - new Date(resource.lastSeenAt).getTime() >
    thresholdSeconds * 1000
  );
}

export function meterValue(value: number | null | undefined): number | null {
  if (value == null || !Number.isFinite(value)) return null;
  return Math.min(100, Math.max(0, value));
}

export function formatUptime(seconds: number | null | undefined): string {
  if (seconds == null || !Number.isFinite(seconds) || seconds < 0) return '—';
  const whole = Math.floor(seconds);
  const days = Math.floor(whole / 86_400);
  const hours = Math.floor((whole % 86_400) / 3_600);
  const minutes = Math.floor((whole % 3_600) / 60);
  if (days) return `${days}d ${hours}h`;
  if (hours) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
}
