// @vitest-environment node
import { describe, it, expect } from 'vitest';
import { queryKeys } from './queryKeys';

describe('queryKeys dashboard stats', () => {
  it('stats returns key with complexId', () => {
    expect(queryKeys.dashboard.stats('c-1')).toEqual(['dashboard', 'stats', 'c-1']);
  });

  it('revenue returns key with complexId and period', () => {
    expect(queryKeys.dashboard.revenue('c-1', 'week')).toEqual(['dashboard', 'revenue', 'c-1', 'week']);
  });

  it('occupancy returns key with complexId', () => {
    expect(queryKeys.dashboard.occupancy('c-1')).toEqual(['dashboard', 'occupancy', 'c-1']);
  });

  it('clients returns key with complexId', () => {
    expect(queryKeys.dashboard.clients('c-1')).toEqual(['dashboard', 'clients', 'c-1']);
  });
});
