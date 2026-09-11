import { useContext, useEffect } from 'react';
import { PageHeadingContext, type PageHeading } from './pageHeadingContextObject';

/**
 * Pages call this to publish their title (rendered in the desktop masthead, in
 * line with the logo, always paired with the live date/time) — mirrors
 * `usePageTitle` for the document title.
 */
export function usePageHeading(title: string) {
  // Only `setHeading` (not the whole context value) is a dependency: it's the
  // raw useState setter, stable across renders — the value object changes on
  // every heading update, which would otherwise re-fire this effect forever.
  const setHeading = useContext(PageHeadingContext)?.setHeading;

  useEffect(() => {
    setHeading?.({ title });
    return () => {
      setHeading?.(null);
    };
  }, [setHeading, title]);
}

/** The shell reads the currently published heading to render in the masthead. */
export function useCurrentPageHeading(): PageHeading | null {
  return useContext(PageHeadingContext)?.heading ?? null;
}
