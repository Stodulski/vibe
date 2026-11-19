// @vitest-environment node
import { describe, it, expect } from 'vitest';
import { STORAGE_KEYS } from './storageKeys';

describe('STORAGE_KEYS', () => {
  // Two keys sharing a value would silently overwrite each other's stored data.
  it('all values are unique', () => {
    const values = Object.values(STORAGE_KEYS);
    const uniqueValues = new Set(values);
    expect(uniqueValues.size).toBe(values.length);
  });
});
