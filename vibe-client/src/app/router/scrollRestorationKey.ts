import type { Location } from 'react-router-dom';

/**
 * The key `ScrollToTop` gives `<ScrollRestoration getKey={...} />`.
 *
 * A separate module (not inline in `ScrollToTop.tsx`) for two reasons: the
 * `react-refresh/only-export-components` rule wants component files to export
 * only components, and `ScrollRestoration` needs a data router to mount at
 * all, which makes it awkward to exercise directly in a unit test — this
 * pure function is what the test actually needs to assert.
 *
 * Path only, no search: the public booking flow keeps every answer in the
 * query (`useComplexPageState`), so a day/duration/hour tap is a
 * `setSearchParams` call, which React Router treats as a new location by
 * default — every choice in that flow was scrolling the page back to the top
 * under the visitor's finger, because "new location" and "new pathname" were
 * being treated as the same thing. They are not. Keying on the pathname alone
 * keeps both of `ScrollToTop`'s behaviours: a real route change (different
 * pathname) still scrolls to the top, and going back still restores the
 * offset, because within one pathname there is only ever one key to restore.
 * Do not go back to the bare `<ScrollRestoration />` default — that
 * reintroduces the jump-to-top-on-every-tap bug.
 */
export function getScrollRestorationKey(location: Pick<Location, 'pathname'>): string {
  return location.pathname;
}
