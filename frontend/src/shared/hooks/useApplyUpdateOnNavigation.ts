import { useEffect, useRef } from 'react';
import { useLocation } from 'react-router-dom';
import { applyPendingServiceWorkerUpdate, isUpdatePending } from '@/shared/lib/serviceWorkerUpdate';
import { hasUnsavedWork } from '@/shared/lib/unsavedWork';

/**
 * Spends a pending service worker update on the next in-app navigation.
 *
 * A new build that waits for a click reaches almost nobody: people keep a tab
 * open for days and never read a toast. A new build that reloads on its own
 * reaches everyone and interrupts them. A route change is the moment both
 * problems disappear — the screen is being replaced anyway, so a reload costs
 * the person nothing they were not already losing.
 *
 * Only the pathname counts. A `?tab=` or a `#section` stays on the same screen
 * with the same state, exactly the rule `useUnsavedChangesBlocker` uses, and
 * reloading on one of those would interrupt someone who never left.
 *
 * Unsaved work is checked all the same. By the time this effect runs the
 * blocker has already let the navigation through, so in practice nothing is
 * dirty any more; the check costs nothing and does not depend on that ordering
 * staying true.
 *
 * The pending flag is read here rather than subscribed to: this reacts to the
 * navigation, not to a new build appearing. A build that lands while someone
 * reads one screen must wait for that person to move, which is the whole
 * point.
 */
export function useApplyUpdateOnNavigation(): void {
  const { pathname } = useLocation();
  const previousPathname = useRef(pathname);

  useEffect(() => {
    if (previousPathname.current === pathname) {
      return;
    }
    previousPathname.current = pathname;

    if (!isUpdatePending() || hasUnsavedWork()) {
      return;
    }
    applyPendingServiceWorkerUpdate();
  }, [pathname]);
}
