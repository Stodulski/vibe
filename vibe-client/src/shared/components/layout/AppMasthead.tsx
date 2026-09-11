import type { ReactNode } from 'react';
import { useCurrentPageHeading } from './page-heading/usePageHeading';
import { useLiveClock } from '@/shared/hooks/useLiveClock';
import { PageContentWidth } from './PageContentWidth';

interface AppMastheadProps {
  /** Logo (+ optional brand label) — matches the sidebar's permanent icon-only width. */
  brand: ReactNode;
}

/**
 * Full-width desktop bar: a brand cell sized to match the (permanently
 * collapsed) sidebar below it, plus the current page's title with the live
 * date/time underneath — same subtitle on every page, published via
 * `usePageHeading`. Kept as its own component so `AppShell` stays under the
 * repo's max-lines-per-function cap.
 */
export function AppMasthead({ brand }: AppMastheadProps) {
  const heading = useCurrentPageHeading();
  const clock = useLiveClock();

  return (
    <div className="hidden h-16 shrink-0 items-stretch border-b border-border-subtle bg-bg-base md:flex">
      <div className="flex w-[68px] shrink-0 items-center justify-center">{brand}</div>

      {heading && (
        <div className="flex min-w-0 flex-1 items-center border-l border-border-subtle px-3 sm:px-6 lg:px-10 xl:px-14">
          <PageContentWidth className="flex min-w-0 flex-col justify-center gap-0.5">
            <h1 className="truncate text-sm font-semibold text-text-primary sm:text-base">{heading.title}</h1>
            <p className="truncate text-xs text-text-tertiary first-letter:uppercase">{clock}</p>
          </PageContentWidth>
        </div>
      )}
    </div>
  );
}
