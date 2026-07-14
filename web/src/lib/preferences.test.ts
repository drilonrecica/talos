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
  it('defaults first-time users to the signature dark theme', () => {
    const storage = { getItem: () => null } as unknown as Storage;
    expect(preferences(storage)).toEqual({
      schemaVersion: 1,
      theme: 'dark',
      density: 'comfortable',
      pinnedResources: [],
      landingPage: 'watch',
      chartRange: '24h',
    });
  });

  it('reads Binnacle-scoped keys', () => {
    const values = new Map([
      ['binnacle.theme', 'dark'],
      ['binnacle.density', 'compact'],
    ]);
    const storage = {
      getItem: (key: string) => values.get(key) ?? null,
    } as Storage;

    expect(preferences(storage)).toEqual({
      schemaVersion: 1,
      theme: 'dark',
      density: 'compact',
      pinnedResources: [],
      landingPage: 'watch',
      chartRange: '24h',
    });
  });

  it('reads a validated server mirror and rejects malformed pins', () => {
    const valid = {
      schemaVersion: 1,
      theme: 'light',
      density: 'compact',
      pinnedResources: ['resource-2'],
      landingPage: 'events',
      chartRange: '7d',
    };
    const storage = {
      getItem: (key: string) =>
        key === 'binnacle.preferences.v1' ? JSON.stringify(valid) : null,
    } as Storage;
    expect(preferences(storage)).toEqual(valid);
  });
});
