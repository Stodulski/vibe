/**
 * One read for every request that asks for the same key at once. A complex's kit is four
 * pictures the panel asks for together; each render holds the function's thread for seconds,
 * so four separate API reads would wait behind each other's renders and time out. They share
 * one read instead, kept for `ttlMs`. A failed read is forgotten at once, so the next request
 * asks the API again.
 */
export function sharedRead<T>(
  read: (key: string) => Promise<T>,
  ttlMs: number,
  now: () => number = Date.now,
): (key: string) => Promise<T> {
  const reads = new Map<string, { value: Promise<T>; expires: number }>();
  return (key) => {
    const kept = reads.get(key);
    if (kept && kept.expires > now()) return kept.value;
    const value = read(key);
    reads.set(key, { value, expires: now() + ttlMs });
    value.catch(() => {
      if (reads.get(key)?.value === value) reads.delete(key);
    });
    return value;
  };
}
