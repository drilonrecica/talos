import { describe, expect, it } from 'vitest';
import { preferences, resolveTheme } from './preferences';

describe('theme resolution', () => {
  it('uses operating-system preference only for system theme', () => {
    expect(resolveTheme('system', true)).toBe('dark');
    expect(resolveTheme('system', false)).toBe('light');
    expect(resolveTheme('light', true)).toBe('light');
  });
});

describe('preference storage', () => {
  it('reads Binnacle-scoped keys', () => {
    const values = new Map([
      ['binnacle.theme', 'dark'],
      ['binnacle.density', 'compact'],
    ]);
    const storage = {
      getItem: (key: string) => values.get(key) ?? null,
    } as Storage;

    expect(preferences(storage)).toEqual({ theme: 'dark', density: 'compact' });
  });
});
