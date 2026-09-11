// @vitest-environment node
import { describe, it, expect } from 'vitest';
import { MAX_VISIBLE_BOOKINGS, CLIENTS_PAGE_SIZE } from './constants';

describe('constants', () => {
  it('MAX_VISIBLE_BOOKINGS is 5', () => {
    expect(MAX_VISIBLE_BOOKINGS).toBe(5);
  });

  it('CLIENTS_PAGE_SIZE is 50', () => {
    expect(CLIENTS_PAGE_SIZE).toBe(50);
  });

  it('all constants are positive integers', () => {
    expect(Number.isInteger(MAX_VISIBLE_BOOKINGS)).toBe(true);
    expect(MAX_VISIBLE_BOOKINGS).toBeGreaterThan(0);

    expect(Number.isInteger(CLIENTS_PAGE_SIZE)).toBe(true);
    expect(CLIENTS_PAGE_SIZE).toBeGreaterThan(0);
  });
});
