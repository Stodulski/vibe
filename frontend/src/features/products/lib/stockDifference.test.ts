import { describe, it, expect } from 'vitest';
import { stockDifference } from './stockDifference';

describe('stockDifference', () => {
  it('is undefined while nothing was typed', () => {
    expect(stockDifference(undefined, 20)).toBeUndefined();
  });

  it('is undefined for a NaN (invalid/blank) counted value', () => {
    expect(stockDifference(Number.NaN, 20)).toBeUndefined();
  });

  it('is positive when the count is higher than current stock', () => {
    expect(stockDifference(25, 20)).toBe(5);
  });

  it('is negative when the count is lower than current stock', () => {
    expect(stockDifference(15, 20)).toBe(-5);
  });

  it('is 0 when the count matches current stock exactly', () => {
    expect(stockDifference(20, 20)).toBe(0);
  });

  it('handles a negative current stock (needs_stock_review case)', () => {
    expect(stockDifference(0, -3)).toBe(3);
  });
});
