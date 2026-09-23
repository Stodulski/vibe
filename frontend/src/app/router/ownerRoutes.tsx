import { Outlet, type RouteObject } from 'react-router-dom';
import { ProtectedRoute } from './ProtectedRoute';
import { SkeletonDashboard, SkeletonSettings, SkeletonBookings } from '@/shared/components/common/Skeletons';
import { lazyPage, lazyShell, ownerPage } from './routeHelpers';
import { PageLoader } from './loaders';

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
      <ProtectedRoute>
        {lazyShell(() => import('@/app/layout/DashboardLayout').then((m) => ({ default: m.DashboardLayout })))}
      </ProtectedRoute>
    ),
    children: [
      {
        path: '/dashboard',
        element: ownerPage(() => import('@/features/dashboard/pages/DashboardPage'), <SkeletonDashboard />),
      },
      {
        path: '/bookings',
        element: ownerPage(() => import('@/features/bookings/pages/BookingsPage'), <SkeletonBookings />),
      },
      {
        path: '/cash',
        element: ownerPage(() => import('@/features/cash/pages/CashPage')),
      },
      {
        path: '/cash/sell',
        element: ownerPage(() => import('@/features/cash/pages/CashSellPage')),
      },
      {
        path: '/cash/sessions/:sessionId',
        element: ownerPage(() => import('@/features/cash/pages/CashSessionDetailPage')),
      },
      {
        path: '/cash/products',
        element: ownerPage(() => import('@/features/products/pages/ProductsPage')),
      },
      {
        path: '/cash/products/:productId',
        element: ownerPage(() => import('@/features/products/pages/ProductDetailPage')),
      },
      {
        path: '/courts',
        element: ownerPage(() => import('@/features/courts/pages/CourtsPage')),
      },
      {
        path: '/clients',
        element: ownerPage(() => import('@/features/clients/pages/ClientsPage')),
      },
      {
        path: '/reports',
        element: ownerPage(() => import('@/features/dashboard/pages/ReportsPage')),
      },
      {
        path: '/settings',
        element: ownerPage(() => import('@/features/complex/pages/SettingsPage'), <SkeletonSettings />),
      },
      {
        path: '/profile',
        element: ownerPage(() => import('@/features/auth/pages/ProfilePage')),
      },
    ],
  },
];
