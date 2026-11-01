import { useEffect, useState } from 'react';
import { venueDateString } from '../lib/venueDate';

/**
 * An instant to read the venue's today from, refreshed when the visitor
 * returns to the tab on a later venue day.
 *
 * Reading `new Date()` once at mount left a tab opened yesterday asking the
 * API for yesterday's grid, which it refuses with 422. The returned `Date`
 * keeps its identity for as long as the venue day is the same, so it is safe
 * as a memo or effect dependency.
 */
export function useVenueNow(): Date {
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    const refresh = () => {
      if (document.visibilityState !== 'visible') return;
      const next = new Date();
      setNow((prev) => (venueDateString(prev) === venueDateString(next) ? prev : next));
    };
    document.addEventListener('visibilitychange', refresh);
    window.addEventListener('focus', refresh);
    window.addEventListener('pageshow', refresh);
    return () => {
      document.removeEventListener('visibilitychange', refresh);
      window.removeEventListener('focus', refresh);
      window.removeEventListener('pageshow', refresh);
    };
  }, []);

  return now;
}
