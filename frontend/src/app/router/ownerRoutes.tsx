import { Outlet, type RouteObject } from 'react-router-dom';
import { ProtectedRoute } from './ProtectedRoute';
import { lazyPage, lazyShell, ownerPage } from './routeHelpers';
import { routeSkeleton } from './routeSkeletons';
import { PageLoader } from './loaders';
import { AppShellLoader } from '@/app/layout/AppShellLoader';

// One pathless guard instead of a `<ProtectedRoute>` per element: the three
// pages below share exactly the same guard, and repeating it remounted the
// auth check on every navigation between them.
export const ownerStandaloneRoutes: RouteObject[] = [
  {
    element: (
      <ProtectedRoute>
        <Outlet />
      </ProtectedRoute>
    ),
    children: [
      {
        path: '/onboarding',
        element: lazyPage(() => import('@/features/onboarding/pages/OnboardingPage'), <PageLoader />),
      },
      {
        path: '/settings/mp/callback',
        element: lazyPage(() => import('@/features/complex/pages/MPCallbackPage'), <PageLoader />),
      },
    ],
  },
];

export const ownerDashboardRoutes: RouteObject[] = [
  {
    element: (
      <ProtectedRoute loadingFallback={<AppShellLoader />}>
        {lazyShell(
          () => import('@/app/layout/DashboardLayout').then((m) => ({ default: m.DashboardLayout })),
          <AppShellLoader />,
        )}
      </ProtectedRoute>
    ),
    children: [
      {
        path: '/dashboard',
        element: ownerPage(() => import('@/features/dashboard/pages/DashboardPage'), routeSkeleton('/dashboard')),
      },
      {
        path: '/bookings',
        element: ownerPage(() => import('@/features/bookings/pages/BookingsPage'), routeSkeleton('/bookings')),
      },
      {
        path: '/open-play',
        element: ownerPage(() => import('@/features/events/pages/EventsPage'), routeSkeleton('/open-play')),
      },
      {
        path: '/cash',
        element: ownerPage(() => import('@/features/cash/pages/CashPage'), routeSkeleton('/cash')),
      },
      {
        path: '/cash/sell',
        element: ownerPage(() => import('@/features/cash/pages/CashSellPage'), routeSkeleton('/cash/sell')),
      },
      {
        path: '/cash/sessions/:sessionId',
        element: ownerPage(
          () => import('@/features/cash/pages/CashSessionDetailPage'),
          routeSkeleton('/cash/sessions/:sessionId'),
        ),
      },
      {
        path: '/cash/products',
        element: ownerPage(() => import('@/features/products/pages/ProductsPage'), routeSkeleton('/cash/products')),
      },
      {
        path: '/cash/products/:productId',
        element: ownerPage(
          () => import('@/features/products/pages/ProductDetailPage'),
          routeSkeleton('/cash/products/:productId'),
        ),
      },
      {
        path: '/courts',
        element: ownerPage(() => import('@/features/courts/pages/CourtsPage'), routeSkeleton('/courts')),
      },
      {
        path: '/clients',
        element: ownerPage(() => import('@/features/clients/pages/ClientsPage'), routeSkeleton('/clients')),
      },
      {
        path: '/reports',
        element: ownerPage(() => import('@/features/dashboard/pages/ReportsPage'), routeSkeleton('/reports')),
      },
      {
        path: '/settings',
        element: ownerPage(() => import('@/features/complex/pages/SettingsPage'), routeSkeleton('/settings')),
      },
      {
        path: '/profile',
        element: ownerPage(() => import('@/features/auth/pages/ProfilePage'), routeSkeleton('/profile')),
      },
    ],
  },
];
