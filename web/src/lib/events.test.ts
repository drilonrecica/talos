import { describe, expect, it } from 'vitest';
import { eventHistoryURL, eventRangeFor, fetchEventHistory } from './events';

describe('event history helpers', () => {
  it('computes ranges from now', () => {
    const now = new Date('2026-07-12T12:00:00.000Z');
    const { from, to } = eventRangeFor('24h', now);
    expect(to.toISOString()).toBe('2026-07-12T12:00:00.000Z');
    expect(from.toISOString()).toBe('2026-07-11T12:00:00.000Z');
  });

  it('builds host event URL', () => {
    const from = new Date('2026-07-11T12:00:00.000Z');
    const to = new Date('2026-07-12T12:00:00.000Z');
    expect(eventHistoryURL('host', undefined, from, to)).toBe(
      '/api/v1/events?from=2026-07-11T12%3A00%3A00.000Z&to=2026-07-12T12%3A00%3A00.000Z',
    );
  });

  it('builds resource-scoped event URL', () => {
    const from = new Date('2026-07-11T12:00:00.000Z');
    const to = new Date('2026-07-12T12:00:00.000Z');
    expect(eventHistoryURL('resource', 'res_abc', from, to)).toBe(
      '/api/v1/events?from=2026-07-11T12%3A00%3A00.000Z&to=2026-07-12T12%3A00%3A00.000Z&resource_id=res_abc',
    );
  });

  it('fetches and parses event history', async () => {
    const payload = [
      {
        id: 'evt-1',
        ts: '2026-07-12T11:00:00.000Z',
        type: 'container_oom',
        severity: 'critical',
        summary: 'oom',
        source: 'docker',
      },
    ];
    // eslint-disable-next-line @typescript-eslint/no-unused-vars
    const fetcher = (_input: RequestInfo | URL, _init?: RequestInit) =>
      Promise.resolve({
        ok: true,
        json: () => Promise.resolve(payload),
      } as Response);
    const from = new Date('2026-07-11T12:00:00.000Z');
    const to = new Date('2026-07-12T12:00:00.000Z');
    const events = await fetchEventHistory(
      'host',
      undefined,
      from,
      to,
      undefined,
      fetcher,
    );
    expect(events).toHaveLength(1);
    expect(events[0].severity).toBe('critical');
  });
});
