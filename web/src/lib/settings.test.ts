import { describe, expect, it } from 'vitest';
import { aggressiveInterval } from './settings';

describe('settings warnings', () => {
  it('warns only for valid sub-default collection intervals', () => {
    expect(aggressiveInterval('1s')).toBe(true);
    expect(aggressiveInterval('1500ms')).toBe(true);
    expect(aggressiveInterval('2s')).toBe(false);
    expect(aggressiveInterval('invalid')).toBe(false);
  });
});
