import { describe, expect, it } from 'vitest';
import {
  formatUptime,
  meterValue,
  prioritizedResources,
  staleResource,
} from './watch';

describe('production watch prioritization', () => {
  it('places unhealthy resources first and identifies stale values', () => {
    const values = prioritizedResources([
      { id: 'a', name: 'Healthy', status: 'healthy' },
      { id: 'b', name: 'Broken', status: 'down' },
    ]);
    expect(values[0].name).toBe('Broken');
    expect(
      staleResource(
        {
          id: 'a',
          name: 'A',
          status: 'healthy',
          lastSeenAt: '2026-07-11T11:59:00Z',
        },
        '2026-07-11T12:00:00Z',
      ),
    ).toBe(true);
  });

  it('bounds meters and formats missing or measured uptime', () => {
    expect(meterValue(-2)).toBe(0);
    expect(meterValue(140)).toBe(100);
    expect(meterValue(null)).toBeNull();
    expect(formatUptime(null)).toBe('—');
    expect(formatUptime(93_600)).toBe('1d 2h');
  });

  it('places valid pins first and preserves health ordering for the rest', () => {
    const values = prioritizedResources(
      [
        { id: 'healthy', name: 'Healthy', status: 'healthy' },
        { id: 'down', name: 'Down', status: 'down' },
        { id: 'pinned', name: 'Pinned', status: 'healthy' },
      ],
      ['missing', 'pinned'],
    );
    expect(values.map((value) => value.id)).toEqual([
      'pinned',
      'down',
      'healthy',
    ]);
  });
});
