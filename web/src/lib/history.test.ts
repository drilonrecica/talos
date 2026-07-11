import { describe, expect, it } from 'vitest';
import { chartPoints, rangeFor, validateRange } from './history';

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
});
