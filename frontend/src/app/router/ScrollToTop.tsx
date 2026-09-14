import { Outlet, ScrollRestoration } from 'react-router-dom';
import { getScrollRestorationKey } from './scrollRestorationKey';
import { useApplyUpdateOnNavigation } from '@/shared/hooks/useApplyUpdateOnNavigation';

/**
 * A pathless root that resets the scroll position on navigation.
 *
 * Without it, a single-page app keeps the previous screen's scroll offset
 * across a route change, so you arrive part-way down a page you have never
 * seen. Measured on the booking flow: pressing "Continuar" from the slot grid
 * landed on the confirm form at scroll 111, with its progress indicator —
 * the one thing on that screen that says where you are — sitting 15px above
 * the top of the viewport. The same happened on every navigation in the app;
 * that flow is only where it was noticed.
 *
 * `ScrollRestoration` rather than a `scrollTo(0, 0)` effect: it scrolls to the
 * top for a new location AND puts the old offset back when someone goes back,
 * which is what a browser does with real pages and what the effect version
 * would have taken away.
 *
 * It wraps every route, including the auth pages that have no layout of their
 * own, so nothing is left out by being added to a layout later.
 *
 * That same reach is why the PWA update trigger is mounted here rather than in
 * a layout: `router.tsx` puts this component above every route group — auth,
 * owner, admin, the public booking flow and the 404 — so a pending build is
 * picked up on any navigation in the app, not only inside the dashboard. See
 * `useApplyUpdateOnNavigation`.
 *
 * See `getScrollRestorationKey` for why it is keyed on the pathname alone
 * rather than the default `ScrollRestoration` key — that is what keeps every
 * choice in the public booking flow from scrolling the page back to the top.
 */
export function ScrollToTop() {
  useApplyUpdateOnNavigation();

  return (
    <>
      <ScrollRestoration getKey={getScrollRestorationKey} />
      <Outlet />
    </>
  );
}
