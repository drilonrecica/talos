export type HistoricalEvent = {
  id: string;
  ts: string;
  type: string;
  severity: 'info' | 'warning' | 'critical';
  summary: string;
  details?: string;
  correlationKey?: string;
  containerInstanceId?: string;
  resourceId?: string;
  source: string;
};

export type EventRangeKey = '1h' | '6h' | '24h' | '7d';

const durations: Record<EventRangeKey, number> = {
  '1h': 3600_000,
  '6h': 6 * 3600_000,
  '24h': 24 * 3600_000,
  '7d': 7 * 24 * 3600_000,
};

export function eventRangeFor(
  key: EventRangeKey,
  now = new Date(),
): { from: Date; to: Date } {
  return { from: new Date(now.getTime() - durations[key]), to: now };
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

export async function fetchEventHistory(
  scope: 'host' | 'resource',
  id: string | undefined,
  from: Date,
  to: Date,
  signal?: AbortSignal,
  fetcher = fetch,
): Promise<HistoricalEvent[]> {
  const response = await fetcher(eventHistoryURL(scope, id, from, to), {
    credentials: 'same-origin',
    signal,
  });
  if (!response.ok) throw new Error('Event history is unavailable.');
  return response.json() as Promise<HistoricalEvent[]>;
}
