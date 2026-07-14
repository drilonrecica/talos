import { authenticatedMutation } from './auth';

export type Theme = 'system' | 'dark' | 'light';
export type Density = 'comfortable' | 'compact';
export type LandingPage =
  'watch' | 'resources' | 'server' | 'events' | 'alerts';
export type ChartRange = '1h' | '6h' | '24h' | '7d' | '30d';

export interface UserPreferences {
  schemaVersion: 1;
  theme: Theme;
  density: Density;
  pinnedResources: string[];
  landingPage: LandingPage;
  chartRange: ChartRange;
  updatedAt?: string;
}

const themeKey = 'binnacle.theme';
const densityKey = 'binnacle.density';
const mirrorKey = 'binnacle.preferences.v1';

const defaults: UserPreferences = {
  schemaVersion: 1,
  theme: 'dark',
  density: 'comfortable',
  pinnedResources: [],
  landingPage: 'watch',
  chartRange: '24h',
};

export function resolveTheme(
  theme: Theme,
  dark = matchMedia('(prefers-color-scheme: dark)').matches,
): 'dark' | 'light' {
  return theme === 'system' ? (dark ? 'dark' : 'light') : theme;
}

export function preferences(storage: Storage = localStorage): UserPreferences {
  try {
    const mirror = JSON.parse(storage.getItem(mirrorKey) ?? 'null') as unknown;
    if (validPreferences(mirror)) return mirror;
  } catch {
    // Fall through to the legacy keys for the one-time server migration.
  }
  const theme = storage.getItem(themeKey);
  const density = storage.getItem(densityKey);
  return {
    ...defaults,
    theme:
      theme === 'dark' || theme === 'light' || theme === 'system'
        ? theme
        : defaults.theme,
    density:
      density === 'compact' || density === 'comfortable'
        ? density
        : defaults.density,
  };
}

export function applyPreferences(
  value: UserPreferences,
  storage: Storage = localStorage,
) {
  storage.setItem(themeKey, value.theme);
  storage.setItem(densityKey, value.density);
  storage.setItem(mirrorKey, JSON.stringify(value));
  document.documentElement.dataset.theme = resolveTheme(value.theme);
  document.documentElement.dataset.density = value.density;
  window.dispatchEvent(
    new CustomEvent('binnacle:preferences', { detail: value }),
  );
}

export async function loadServerPreferences(): Promise<UserPreferences> {
  const response = await fetch('/api/v1/preferences', {
    credentials: 'same-origin',
  });
  if (!response.ok) throw new Error('Preferences could not be loaded.');
  const body = (await response.json()) as {
    exists: boolean;
    preferences?: UserPreferences;
  };
  const value =
    body.exists && validPreferences(body.preferences)
      ? body.preferences
      : await saveServerPreferences(preferences());
  applyPreferences(value);
  return value;
}

export async function saveServerPreferences(
  value: UserPreferences,
): Promise<UserPreferences> {
  if (!validPreferences(value)) throw new Error('Preferences are invalid.');
  const saved = await authenticatedMutation<UserPreferences>(
    '/api/v1/preferences',
    'PUT',
    value,
  );
  if (!saved || !validPreferences(saved))
    throw new Error('Preferences could not be saved.');
  applyPreferences(saved);
  return saved;
}

function validPreferences(value: unknown): value is UserPreferences {
  if (!value || typeof value !== 'object') return false;
  const candidate = value as Partial<UserPreferences>;
  return (
    candidate.schemaVersion === 1 &&
    ['system', 'dark', 'light'].includes(candidate.theme ?? '') &&
    ['comfortable', 'compact'].includes(candidate.density ?? '') &&
    ['watch', 'resources', 'server', 'events', 'alerts'].includes(
      candidate.landingPage ?? '',
    ) &&
    ['1h', '6h', '24h', '7d', '30d'].includes(candidate.chartRange ?? '') &&
    Array.isArray(candidate.pinnedResources) &&
    candidate.pinnedResources.length <= 12 &&
    new Set(candidate.pinnedResources).size ===
      candidate.pinnedResources.length &&
    candidate.pinnedResources.every(
      (id) => typeof id === 'string' && id.length > 0 && id.length <= 128,
    )
  );
}
