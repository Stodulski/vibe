import { renderHook } from '@testing-library/react';
import { useIdempotencyKey } from './idempotency';

/**
 * The whole point of the key is that it is stable across the retries of one
 * submit and different for a new one — the backend replays the stored answer
 * for a repeated key and refuses a repeated key with a different body, so a
 * key that changed per retry would defeat the deduplication and a key that
 * never changed would make the second, deliberate booking a 409.
 */
describe('useIdempotencyKey', () => {
  it('hands out the same key for every read within one attempt', () => {
    const { result } = renderHook(() => useIdempotencyKey());

    const key = result.current.begin();
    expect(result.current.current()).toBe(key);
    expect(result.current.current()).toBe(key);
  });

  it('mints a different key for the next attempt', () => {
    const { result } = renderHook(() => useIdempotencyKey());

    const first = result.current.begin();
    const second = result.current.begin();

    expect(second).not.toBe(first);
    expect(result.current.current()).toBe(second);
  });

  it('survives a re-render without changing the key in flight', () => {
    const { result, rerender } = renderHook(() => useIdempotencyKey());

    const key = result.current.begin();
    rerender();

    expect(result.current.current()).toBe(key);
  });

  it('produces a UUID, which is inside the backend 64-character limit', () => {
    const { result } = renderHook(() => useIdempotencyKey());
    const key = result.current.begin();

    expect(key).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/);
    expect(key.length).toBeLessThanOrEqual(64);
  });
});
