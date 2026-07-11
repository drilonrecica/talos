import { authenticatedMutation } from './auth';

export interface SettingValue {
  value: string;
  source: 'Default' | 'Config file' | 'Environment' | 'Admin override';
  applyMode: 'live' | 'restart_required';
}
export interface SettingsSnapshot {
  revision: number;
  values: Record<string, SettingValue>;
}

export async function loadSettings(): Promise<SettingsSnapshot> {
  const response = await fetch('/api/v1/settings', {
    credentials: 'same-origin',
  });
  if (!response.ok) throw new Error('Settings are unavailable.');
  return (await response.json()) as SettingsSnapshot;
}

export async function patchSetting(
  revision: number,
  key: string,
  value: string,
): Promise<SettingsSnapshot> {
  const snapshot = await authenticatedMutation<SettingsSnapshot>(
    '/api/v1/settings',
    'PATCH',
    {
      revision,
      changes: { [key]: value },
    },
  );
  if (!snapshot) throw new Error('The settings response was empty.');
  return snapshot;
}

export function aggressiveInterval(value: string): boolean {
  const match = /^(\d+(?:\.\d+)?)(ms|s)$/.exec(value.trim());
  if (!match) return false;
  const milliseconds = Number(match[1]) * (match[2] === 's' ? 1000 : 1);
  return milliseconds < 2000;
}
