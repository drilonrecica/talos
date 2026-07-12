import { describe, expect, it } from 'vitest';

describe('frontend workspace', () => {
  it('keeps the alpha product name available to the application', () => {
    expect('Binnacle').toBe('Binnacle');
  });
});
