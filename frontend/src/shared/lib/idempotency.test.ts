import { renderHook, waitFor } from '@testing-library/react';
import { createQueryWrapper } from '@/test/test-utils';
import { useIdempotentMutation, type WithAttemptKey } from './idempotency';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

/**
 * The key has to be stable across the retries of one submit and different for
 * a new one — the backend replays the stored answer for a repeated key and
 * refuses a repeated key carrying a different body. Putting it in the
 * variables is what gets both: React Query hands the same variables object to
 * every retry of one `mutate()`, and never shares it between two of them.
 */
describe('useIdempotentMutation', () => {
  it('adds a UUID attemptKey to the variables the mutationFn receives', async () => {
    const seen: WithAttemptKey<{ id: string }>[] = [];
    const { result } = renderHook(
      () =>
        useIdempotentMutation({
          mutationFn: (variables: WithAttemptKey<{ id: string }>) => {
            seen.push(variables);
            return Promise.resolve('ok');
          },
        }),
      { wrapper: createQueryWrapper() },
    );

    // Called with the bare variables: `attemptKey` is the hook's business,
    // not the call site's, so no consumer had to change.
    result.current.mutate({ id: 'a' });
    await waitFor(() => {
      expect(seen).toHaveLength(1);
    });

    expect(seen[0]?.id).toBe('a');
    expect(seen[0]?.attemptKey).toMatch(UUID);
  });

  it('gives two overlapping calls different keys', async () => {
    const seen: WithAttemptKey<{ id: string }>[] = [];
    let release: (() => void) | undefined;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });

    const { result } = renderHook(
      () =>
        useIdempotentMutation({
          mutationFn: async (variables: WithAttemptKey<{ id: string }>) => {
            seen.push(variables);
            await gate;
            return 'ok';
          },
        }),
      { wrapper: createQueryWrapper() },
    );

    result.current.mutate({ id: 'a' });
    result.current.mutate({ id: 'b' });
    await waitFor(() => {
      expect(seen).toHaveLength(2);
    });
    release?.();

    expect(seen[0]?.id).toBe('a');
    expect(seen[1]?.id).toBe('b');
    expect(seen[1]?.attemptKey).not.toBe(seen[0]?.attemptKey);
  });
});
