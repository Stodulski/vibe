import { createContext, useContext } from 'react';

/**
 * The query string the server saw for this page (`?date=...`, including the
 * leading `?`, or empty). `useQueryParams` reads it where `window` does not
 * exist, and during hydration, so the server markup and the first client render
 * agree. The host passes it in; the package never reads `location` on the server.
 */
export const InitialSearchContext = createContext<string | undefined>(undefined);

/** The query the host seeded, if any. Undefined outside a `BookingRoot` or when the host passed none. */
export function useInitialSearch(): string | undefined {
  return useContext(InitialSearchContext);
}
