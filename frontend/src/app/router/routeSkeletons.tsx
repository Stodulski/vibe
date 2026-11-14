import type { ReactNode } from 'react';
import { matchPath } from 'react-router-dom';
import { ProfilePageSkeleton } from '@/features/auth/pages/ProfilePageSkeleton';
import { BookingsPageSkeleton } from '@/features/bookings/pages/BookingsPageSkeleton';
import { CashPageSkeleton } from '@/features/cash/pages/CashPageSkeleton';
import { CashSellPageSkeleton } from '@/features/cash/pages/CashSellPageSkeleton';
import { CashSessionDetailPageSkeleton } from '@/features/cash/pages/CashSessionDetailPageSkeleton';
import { ClientsPageSkeleton } from '@/features/clients/pages/ClientsPageSkeleton';
import { SettingsPageSkeleton } from '@/features/complex/pages/SettingsPageSkeleton';
import { CourtsPageSkeleton } from '@/features/courts/pages/CourtsPageSkeleton';
import { DashboardPageSkeleton } from '@/features/dashboard/pages/DashboardPageSkeleton';
import { ReportsPageSkeleton } from '@/features/dashboard/pages/ReportsPageSkeleton';
import { EventsPageSkeleton } from '@/features/events/pages/EventsPageSkeleton';
import { ProductDetailPageSkeleton } from '@/features/products/pages/ProductDetailPageSkeleton';
import { ProductsPageSkeleton } from '@/features/products/pages/ProductsPageSkeleton';

/**
 * The one table from owner route pattern to the skeleton its page shows while
 * it loads. `ownerRoutes` takes its `ownerPage` fallbacks from here and
 * `AppShellLoader` picks by pathname, so the shell's placeholder and the route
 * fallback are the same node and the swap between them is invisible.
 *
 * Each page skeleton lives in the feature that owns the page and is composed
 * of the component skeletons next to the components it mirrors.
 */
const OWNER_ROUTE_SKELETONS = {
  '/dashboard': () => <DashboardPageSkeleton />,
  '/bookings': () => <BookingsPageSkeleton />,
  '/open-play': () => <EventsPageSkeleton />,
  '/cash': () => <CashPageSkeleton />,
  '/cash/sell': () => <CashSellPageSkeleton />,
  '/cash/sessions/:sessionId': () => <CashSessionDetailPageSkeleton />,
  '/cash/products': () => <ProductsPageSkeleton />,
  '/cash/products/:productId': () => <ProductDetailPageSkeleton />,
  '/courts': () => <CourtsPageSkeleton />,
  '/clients': () => <ClientsPageSkeleton />,
  '/reports': () => <ReportsPageSkeleton />,
  '/settings': () => <SettingsPageSkeleton />,
  '/profile': () => <ProfilePageSkeleton />,
} satisfies Record<string, () => ReactNode>;

type OwnerSkeletonPattern = keyof typeof OWNER_ROUTE_SKELETONS;

/** Fallback of one owner route, by the same pattern its `path` declares. */
export function routeSkeleton(pattern: OwnerSkeletonPattern): ReactNode {
  return OWNER_ROUTE_SKELETONS[pattern]();
}

/** Skeleton of the owner route `pathname` resolves to, or `null` when none matches. */
export function skeletonForPath(pathname: string): ReactNode {
  const patterns = Object.keys(OWNER_ROUTE_SKELETONS) as OwnerSkeletonPattern[];
  const pattern = patterns.find((p) => matchPath({ path: p, end: true }, pathname));
  return pattern ? routeSkeleton(pattern) : null;
}
