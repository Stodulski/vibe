import type { ReactNode } from 'react';
import { matchPath } from 'react-router-dom';
import { SkeletonBookings, SkeletonDashboard, SkeletonSettings } from '@/shared/components/common/Skeletons';
import { ClientsPageSkeleton } from '@/features/clients/components/skeletons/ClientsPageSkeleton';
import { CourtsPageSkeleton } from '@/features/courts/components/skeletons/CourtsPageSkeleton';
import { ReportsPageSkeleton } from '@/features/dashboard/components/skeletons/ReportsPageSkeleton';
import {
  cashFallback,
  cashSellFallback,
  cashSessionFallback,
  productDetailFallback,
  productsFallback,
} from './routeHelpers';
import { ProfilePageSkeleton } from './skeletons/ProfilePageSkeleton';

/**
 * The one table from owner route pattern to the skeleton its page shows while
 * it loads. `ownerRoutes` takes its `ownerPage` fallbacks from here and
 * `AppShellLoader` picks by pathname, so the shell's placeholder and the route
 * fallback are the same node and the swap between them is invisible.
 */
const OWNER_ROUTE_SKELETONS = {
  '/dashboard': () => <SkeletonDashboard />,
  '/bookings': () => <SkeletonBookings />,
  '/cash': cashFallback,
  '/cash/sell': cashSellFallback,
  '/cash/sessions/:sessionId': cashSessionFallback,
  '/cash/products': productsFallback,
  '/cash/products/:productId': productDetailFallback,
  '/courts': () => <CourtsPageSkeleton />,
  '/clients': () => <ClientsPageSkeleton />,
  '/reports': () => <ReportsPageSkeleton />,
  '/settings': () => <SkeletonSettings />,
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
