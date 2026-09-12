import type { RouteObject } from 'react-router-dom';
import { SkeletonBookConfirm, SkeletonBookSuccess, SkeletonCancelInfo } from '@/shared/components/common/Skeletons';
import { SkeletonComplexHeader } from '@/features/public-booking/components/SkeletonComplexHeader';
import { SkeletonSlotGrid } from '@/features/public-booking/components/SkeletonSlotGrid';
import { lazyPage, lazyShell } from './routeHelpers';
import { PublicPageLoader } from './loaders';

export const publicRoutes: RouteObject[] = [
  {
    element: lazyShell(
      () => import('@/shared/components/layout/PublicLayout').then((m) => ({ default: m.PublicLayout })),
      <PublicPageLoader />,
    ),
    children: [
      {
        path: '/:slug',
        element: lazyPage(
          () => import('@/features/public-booking/pages/ComplexPage'),
          <div className="w-full space-y-6 animate-fade-in sm:space-y-10">
            <SkeletonComplexHeader />
            <SkeletonSlotGrid />
          </div>,
        ),
      },
      {
        // Not a page anyone navigates to — MercadoPago's failure back_url.
        // See BookPage for why it must not be removed.
        path: '/:slug/book',
        element: lazyPage(() => import('@/features/public-booking/pages/BookPage')),
      },
      {
        path: '/:slug/book/confirm',
        element: lazyPage(() => import('@/features/public-booking/pages/BookConfirmPage'), <SkeletonBookConfirm />),
      },
      {
        path: '/:slug/book/success',
        element: lazyPage(() => import('@/features/public-booking/pages/BookSuccessPage'), <SkeletonBookSuccess />),
      },
      {
        path: '/:slug/book/cancel',
        element: lazyPage(() => import('@/features/public-booking/pages/BookCancelPage'), <SkeletonCancelInfo />),
      },
    ],
  },
];
