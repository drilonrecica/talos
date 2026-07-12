export type Metric =
  'cpu' | 'memory' | 'network_rx' | 'network_tx' | 'block_read' | 'block_write';
export type HistoryPoint = {
  at: string;
  min: number | null;
  avg: number | null;
  max: number | null;
  count: number;
};
export type HistorySeries = {
  metric: Metric;
  unit: string;
  points: HistoryPoint[];
};
export type HistoryGap = {
  from: string;
  to: string;
  reason:
    'missing' | 'inactive' | 'collector_unavailable' | 'persistence_failure';
};
export type HistoryResponse = {
  scope: 'host' | 'resource';
  id?: string;
  from: string;
  to: string;
  resolution: string;
  series: HistorySeries[];
  gaps: HistoryGap[];
};
export type RangeKey = '1h' | '6h' | '24h' | '7d' | '30d' | 'custom';

const durations: Record<Exclude<RangeKey, 'custom'>, number> = {
  '1h': 3600_000,
  '6h': 6 * 3600_000,
  '24h': 24 * 3600_000,
  '7d': 7 * 24 * 3600_000,
  '30d': 30 * 24 * 3600_000,
};
export function rangeFor(
  key: Exclude<RangeKey, 'custom'>,
  now = new Date(),
): { from: Date; to: Date } {
  return { from: new Date(now.getTime() - durations[key]), to: now };
}
export function validateRange(from: Date, to: Date): string | null {
  if (
    !Number.isFinite(from.getTime()) ||
    !Number.isFinite(to.getTime()) ||
    from >= to
  )
    return 'Choose an end time after the start time.';
  if (to.getTime() - from.getTime() > durations['30d'])
    return 'Custom ranges cannot exceed 30 days.';
  return null;
}

export function chartPoints(series: HistorySeries, gaps: HistoryGap[]) {
  const points = series.points.map((point) => ({
    at: new Date(point.at).getTime() / 1000,
    value: point.avg,
  }));
  for (const gap of gaps) {
    points.push({ at: new Date(gap.from).getTime() / 1000, value: null });
    points.push({ at: new Date(gap.to).getTime() / 1000, value: null });
  }
  return points.sort((a, b) => a.at - b.at);
}
export function eventHistoryURL(
  scope: 'host' | 'resource',
  id: string | undefined,
  from: Date,
  to: Date,
) {
  const query = new URLSearchParams({
    from: from.toISOString(),
    to: to.toISOString(),
  });
  if (scope === 'resource' && id) query.set('resource_id', id);
  return `/api/v1/events?${query}`;
}
export async function fetchHistory(
  scope: 'host' | 'resource',
  id: string | undefined,
  metrics: Metric[],
  from: Date,
  to: Date,
  signal?: AbortSignal,
  fetcher = fetch,
): Promise<HistoryResponse> {
  const query = new URLSearchParams({
    scope,
    metrics: metrics.join(','),
    from: from.toISOString(),
    to: to.toISOString(),
  });
  if (id) query.set('id', id);
  const response = await fetcher(`/api/v1/metrics?${query}`, {
    credentials: 'same-origin',
    signal,
  });
  if (!response.ok) {
    let message = 'Historical telemetry is unavailable.';
    try {
      const body = (await response.json()) as { error?: { message?: string } };
      message = body.error?.message ?? message;
    } catch {
      // Preserve the safe fallback for malformed upstream responses.
    }
    throw new Error(message);
  }
  return response.json() as Promise<HistoryResponse>;
}
