import type { RouteObject } from 'react-router-dom';
import { ProtectedRoute } from './ProtectedRoute';
import { DashboardLayout } from '@/app/layout/DashboardLayout';
import { SkeletonDashboard, SkeletonSettings, SkeletonBookings } from '@/shared/components/common/Skeletons';
import { lazyPage, ownerPage } from './routeHelpers';
import { PageLoader } from './loaders';

export const ownerStandaloneRoutes: RouteObject[] = [
  {
    path: '/complexes',
    element: (
      <ProtectedRoute>{lazyPage(() => import('@/pages/owner/ComplexSelectorPage'), <PageLoader />)}</ProtectedRoute>
    ),
  },
  {
    path: '/onboarding',
    element: <ProtectedRoute>{lazyPage(() => import('@/pages/owner/OnboardingPage'), <PageLoader />)}</ProtectedRoute>,
  },
  {
    path: '/settings/mp/callback',
    element: <ProtectedRoute>{lazyPage(() => import('@/pages/owner/MPCallbackPage'), <PageLoader />)}</ProtectedRoute>,
  },
];

export const ownerDashboardRoutes: RouteObject[] = [
  {
    element: (
      <ProtectedRoute>
        <DashboardLayout />
      </ProtectedRoute>
    ),
    children: [
      {
        path: '/dashboard',
        element: ownerPage(() => import('@/pages/owner/DashboardPage'), <SkeletonDashboard />),
      },
      {
        path: '/bookings',
        element: ownerPage(() => import('@/features/bookings/pages/BookingsPage'), <SkeletonBookings />),
      },
      {
        path: '/courts',
        element: ownerPage(() => import('@/pages/owner/CourtsPage')),
      },
      {
        path: '/clients',
        element: ownerPage(() => import('@/pages/owner/ClientsPage')),
      },
      {
        path: '/reports',
        element: ownerPage(() => import('@/pages/owner/ReportsPage')),
      },
      {
        path: '/settings',
        element: ownerPage(() => import('@/pages/owner/SettingsPage'), <SkeletonSettings />),
      },
      {
        path: '/profile',
        element: ownerPage(() => import('@/features/auth/pages/ProfilePage')),
      },
    ],
  },
];
