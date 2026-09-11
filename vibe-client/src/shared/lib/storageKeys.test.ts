// @vitest-environment node
import { describe, it, expect } from 'vitest';
import { STORAGE_KEYS } from './storageKeys';

describe('STORAGE_KEYS', () => {
  it('has SELECTED_COMPLEX_ID key', () => {
    expect(STORAGE_KEYS.SELECTED_COMPLEX_ID).toBe('selectedComplexId');
  });

  it('has BOOKING_CLIENT_DATA key', () => {
    expect(STORAGE_KEYS.BOOKING_CLIENT_DATA).toBe('vibe_booking_client');
  });

  it('has THEME key', () => {
    expect(STORAGE_KEYS.THEME).toBe('vibe-theme');
  });

  it('has all expected keys defined', () => {
    const keys = Object.keys(STORAGE_KEYS);
    expect(keys).toContain('SELECTED_COMPLEX_ID');
    expect(keys).toContain('BOOKING_CLIENT_DATA');
    expect(keys).toContain('THEME');
  });

  it('all values are non-empty strings', () => {
    for (const value of Object.values(STORAGE_KEYS)) {
      expect(typeof value).toBe('string');
      expect(value.length).toBeGreaterThan(0);
    }
  });

  it('all values are unique', () => {
    const values = Object.values(STORAGE_KEYS);
    const uniqueValues = new Set(values);
    expect(uniqueValues.size).toBe(values.length);
  });
});
