import { Skeleton } from '@/shared/components/ui/skeleton';
import { ES_AR } from '@/shared/i18n/es_AR';
import { mainNavItems, secondaryNavItems } from '@/shared/components/layout/sidebar/navItems';

const NAV_ITEM_COUNT = mainNavItems.length + secondaryNavItems.length;

/**
 * The owner app shell with nothing in it yet: what a hard reload shows while
 * the auth gate, the lazy `DashboardLayout` chunk and the complex query
 * resolve. One component for all three phases so the chrome never changes
 * shape between them.
 *
 * Mirrors `AppShell`'s real geometry: a fixed 64px top bar (brand cell plus
 * title from `md`, the mobile header below it), the 152px sidebar rail from
 * `md`, and `<main>` offset by the same margins and padding.
 */
export function AppShellLoader() {
  return (
    <div className="bg-bg-base relative min-h-dvh" role="status" aria-label={ES_AR.common.loading}>
      <div className="fixed inset-x-0 top-0 z-30" aria-hidden="true">
        {/* Desktop masthead */}
        <div className="border-border-subtle bg-bg-base hidden h-16 items-stretch border-b md:flex">
          <div className="flex w-[152px] shrink-0 items-center justify-center">
            <Skeleton className="h-6 w-20 rounded-lg" />
          </div>
          <div className="border-border-subtle flex min-w-0 flex-1 flex-col justify-center gap-1.5 border-l px-3 sm:px-6 lg:px-10 xl:px-14">
            <Skeleton className="h-4 w-32 rounded-lg" />
            <Skeleton className="h-3 w-44 rounded-lg" />
          </div>
        </div>
        {/* Mobile header */}
        <div className="border-border-subtle bg-bg-base flex h-16 items-center justify-between border-b px-4 md:hidden">
          <Skeleton className="size-10 rounded-lg" />
          <Skeleton className="h-6 w-20 rounded-lg" />
        </div>
      </div>

      {/* Sidebar rail */}
      <div className="fixed top-16 bottom-0 left-0 z-20 hidden md:flex" aria-hidden="true">
        <div className="bg-bg-sidebar border-border-subtle flex h-full w-[152px] shrink-0 flex-col border-r px-3 pt-1">
          <div className="space-y-0.5">
            {Array.from({ length: NAV_ITEM_COUNT }, (_, i) => (
              <div key={i} className="flex items-center rounded-lg px-2.5 py-[7px]">
                <Skeleton className="h-4 w-16 rounded-lg" />
              </div>
            ))}
          </div>
        </div>
      </div>

      <main
        className="relative z-10 mt-16 px-3 py-4 sm:px-6 sm:py-6 md:ml-[152px] lg:px-10 lg:py-8 xl:px-14"
        aria-hidden="true"
      >
        <div className="mx-auto w-full max-w-[1600px]">
          {/* Page title: mobile only, the masthead carries it from `md` */}
          <Skeleton className="mb-3 h-7 w-40 rounded-lg sm:mb-5 md:hidden" />
          <div className="flex flex-col gap-4 md:gap-6">
            <Skeleton className="h-24 w-full max-w-xl rounded-xl" />
            <div className="grid grid-cols-1 gap-4 md:gap-6 lg:grid-cols-2">
              <Skeleton className="h-56 w-full rounded-3xl" />
              <Skeleton className="h-56 w-full rounded-3xl" />
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
