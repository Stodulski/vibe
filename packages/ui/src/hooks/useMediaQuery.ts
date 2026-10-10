import { useSyncExternalStore } from 'react';

/**
 * Whether a CSS media query currently matches.
 *
 * For the cases CSS cannot express: not "style this differently at that
 * width" — which belongs in a class, always — but "render a different
 * component". The public header needs one of those. Below its breakpoint the
 * opening hours and services stack into a column and become an accordion;
 * above it they sit side by side, open, with no control to press. Doing that
 * with CSS would mean shipping both trees and hiding one, which leaves a
 * screen reader with two copies of every heading and a button that does
 * nothing.
 *
 * `useSyncExternalStore` rather than an effect: it reads the current match
 * during render instead of after it, so the first paint is already correct
 * and nothing flashes the wrong layout.
 */
export function useMediaQuery(query: string): boolean {
  return useSyncExternalStore(
    (onChange) => {
      const list = window.matchMedia(query);
      list.addEventListener('change', onChange);
      return () => {
        list.removeEventListener('change', onChange);
      };
    },
    () => window.matchMedia(query).matches,
    // Server snapshot. Nothing renders this on a server today, but the
    // prerender middleware runs the app for crawlers: false keeps that path
    // on the stacked layout, which carries the same content.
    () => false,
  );
}
