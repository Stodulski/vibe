import type { RouteObject } from 'react-router-dom';
import { ProtectedRoute } from './ProtectedRoute';
import { lazyShell, ownerPage } from './routeHelpers';

export const adminRoutes: RouteObject[] = [
  {
    element: (
      <ProtectedRoute allowedRoles={['superadmin']}>
        {lazyShell(() => import('@/app/layout/AdminLayout').then((m) => ({ default: m.AdminLayout })))}
      </ProtectedRoute>
    ),
    children: [
      {
        path: '/admin',
        element: ownerPage(() => import('@/features/admin/pages/AdminDashboardPage')),
      },
      {
        path: '/admin/users',
        element: ownerPage(() => import('@/features/admin/pages/AdminUsersPage')),
      },
      {
        path: '/admin/users/:id',
        element: ownerPage(() => import('@/features/admin/pages/AdminUserDetailPage')),
      },
      {
        path: '/admin/complexes',
        element: ownerPage(() => import('@/features/admin/pages/AdminComplexesPage')),
      },
      {
        path: '/admin/complexes/:id',
        element: ownerPage(() => import('@/features/admin/pages/AdminComplexDetailPage')),
      },
    ],
  },
];
