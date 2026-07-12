export type Theme = 'system' | 'dark' | 'light';
export type Density = 'comfortable' | 'compact';

const themeKey = 'binnacle.theme';
const densityKey = 'binnacle.density';

export function resolveTheme(
  theme: Theme,
  dark = matchMedia('(prefers-color-scheme: dark)').matches,
): 'dark' | 'light' {
  return theme === 'system' ? (dark ? 'dark' : 'light') : theme;
}

export function preferences(storage: Storage = localStorage): {
  theme: Theme;
  density: Density;
} {
  const theme = storage.getItem(themeKey);
  const density = storage.getItem(densityKey);
  return {
    theme:
      theme === 'dark' || theme === 'light' || theme === 'system'
        ? theme
        : 'system',
    density:
      density === 'compact' || density === 'comfortable'
        ? density
        : 'comfortable',
  };
}

export function applyPreferences(
  value: { theme: Theme; density: Density },
  storage: Storage = localStorage,
) {
  storage.setItem(themeKey, value.theme);
  storage.setItem(densityKey, value.density);
  document.documentElement.dataset.theme = resolveTheme(value.theme);
  document.documentElement.dataset.density = value.density;
}
