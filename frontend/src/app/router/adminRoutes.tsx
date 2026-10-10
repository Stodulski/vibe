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
        element: ownerPage(() =>
          import('@/app/router/AdminPlaceholderPage').then((m) => ({ default: m.AdminPlaceholderPage })),
        ),
      },
    ],
  },
];
