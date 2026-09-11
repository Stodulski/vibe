// @vitest-environment node
import { queryClient } from './queryClient';

describe('queryClient', () => {
  it('is an instance of QueryClient', () => {
    expect(queryClient).toBeDefined();
    expect(typeof queryClient.getDefaultOptions).toBe('function');
  });

  it('has correct default staleTime for queries', () => {
    const defaults = queryClient.getDefaultOptions();
    expect(defaults.queries?.staleTime).toBe(5 * 60 * 1000);
  });

  it('has correct default gcTime for queries', () => {
    const defaults = queryClient.getDefaultOptions();
    expect(defaults.queries?.gcTime).toBe(10 * 60 * 1000);
  });

  it('has retry set to 1 for queries', () => {
    const defaults = queryClient.getDefaultOptions();
    expect(defaults.queries?.retry).toBe(1);
  });

  it('has refetchOnWindowFocus disabled', () => {
    const defaults = queryClient.getDefaultOptions();
    expect(defaults.queries?.refetchOnWindowFocus).toBe(false);
  });

  it('has retry set to 0 for mutations', () => {
    const defaults = queryClient.getDefaultOptions();
    expect(defaults.mutations?.retry).toBe(0);
  });
});
