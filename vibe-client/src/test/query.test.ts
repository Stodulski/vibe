import { queryResult } from './query';

describe('queryResult', () => {
  it('wraps data in a success-shaped UseQueryResult', () => {
    const result = queryResult([1, 2, 3]);
    expect(result.data).toEqual([1, 2, 3]);
    expect(result.isLoading).toBe(false);
    expect(result.isSuccess).toBe(true);
    expect(result.status).toBe('success');
  });

  it('allows overriding fields (e.g. isLoading) for loading-state fixtures', () => {
    const result = queryResult(undefined, { isLoading: true, isPending: true, status: 'pending' });
    expect(result.data).toBeUndefined();
    expect(result.isLoading).toBe(true);
    expect(result.status).toBe('pending');
  });
});
