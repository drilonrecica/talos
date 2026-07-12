import { describe, expect, it } from 'vitest';
import {
  chartPoints,
  eventHistoryURL,
  fetchHistory,
  rangeFor,
  validateRange,
} from './history';

describe('history ranges', () => {
  it('uses bounded preset and custom ranges', () => {
    const now = new Date('2026-01-01T00:00:00Z');
    expect(rangeFor('1h', now).from.toISOString()).toBe(
      '2025-12-31T23:00:00.000Z',
    );
    expect(
      validateRange(new Date('2026-01-01'), new Date('2026-02-02')),
    ).toContain('30 days');
  });
  it('inserts null boundaries for explicit gaps', () => {
    const points = chartPoints(
      {
        metric: 'cpu',
        unit: 'percent',
        points: [
          { at: '2026-01-01T00:00:00Z', min: 1, avg: 1, max: 1, count: 1 },
        ],
      },
      [
        {
          from: '2026-01-01T00:01:00Z',
          to: '2026-01-01T00:02:00Z',
          reason: 'collector_unavailable',
        },
      ],
    );
    expect(points.map((point) => point.value)).toEqual([1, null, null]);
  });
  it('scopes resource annotations to the selected range and resource', () => {
    const url = eventHistoryURL(
      'resource',
      'res_test',
      new Date('2026-07-11T11:00:00Z'),
      new Date('2026-07-11T12:00:00Z'),
    );
    expect(url).toContain('resource_id=res_test');
    expect(url).toContain('from=2026-07-11T11%3A00%3A00.000Z');
  });
});

describe('history API errors', () => {
  it('preserves the safe server error message', async () => {
    const fetcher = async () =>
      new Response(
        JSON.stringify({ error: { message: 'Too many metric queries.' } }),
        { status: 429, headers: { 'Content-Type': 'application/json' } },
      );
    await expect(
      fetchHistory(
        'host',
        undefined,
        ['cpu'],
        new Date('2026-01-01T00:00:00Z'),
        new Date('2026-01-01T01:00:00Z'),
        undefined,
        fetcher,
      ),
    ).rejects.toThrow('Too many metric queries.');
  });
});
