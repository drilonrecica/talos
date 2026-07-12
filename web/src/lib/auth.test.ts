import { describe, expect, it, vi } from 'vitest';
import { safeRedirect } from './auth';

describe('safeRedirect', () => {
  it('allows only same-origin application paths', () => {
    vi.stubGlobal('location', { origin: 'https://binnacle.test' });
    expect(safeRedirect('/resources/res_1?range=1h')).toBe(
      '/resources/res_1?range=1h',
    );
    expect(safeRedirect('https://evil.test/steal')).toBe('/overview');
    expect(safeRedirect('//evil.test/steal')).toBe('/overview');
    expect(safeRedirect('/login')).toBe('/overview');
    vi.unstubAllGlobals();
  });
});
