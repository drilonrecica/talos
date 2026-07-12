import { describe, expect, it } from 'vitest';
import { summary, toSeries } from './chart';
describe('chart conversion', () => {
  it('keeps gaps and caps data', () =>
    expect(
      toSeries(
        [
          { at: 1, value: null },
          { at: 2, value: 3 },
        ],
        1,
      ),
    ).toEqual([[2], [3]]));
});

describe('chart statistics', () => {
  it('ignores explicit gaps', () =>
    expect(
      summary([
        { at: 1, value: null },
        { at: 2, value: 3 },
        { at: 3, value: 9 },
      ]),
    ).toEqual({ min: 3, avg: 6, max: 9 }));

  it('returns null when every value is unavailable', () =>
    expect(summary([{ at: 1, value: null }])).toBeNull());
});
