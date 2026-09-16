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
    <div className="border-border-subtle bg-bg-base hidden h-16 shrink-0 items-stretch border-b md:flex">
      <div className="flex w-[68px] shrink-0 items-center justify-center">{brand}</div>

      <div className="border-border-subtle flex min-w-0 flex-1 items-center border-l px-3 sm:px-6 lg:px-10 xl:px-14">
        {heading && (
          <PageContentWidth className="flex min-w-0 flex-col justify-center gap-0.5">
            <h1 className="text-text-primary truncate text-sm font-semibold sm:text-base">{heading.title}</h1>
            <p className="text-text-tertiary truncate text-xs first-letter:uppercase">{clock}</p>
          </PageContentWidth>
        )}
      </div>
    </div>
  );
}
