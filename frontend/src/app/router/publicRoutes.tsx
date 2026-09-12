import type { RouteObject } from 'react-router-dom';
import { PublicLayout } from '@/shared/components/layout/PublicLayout';
import { SkeletonBookConfirm, SkeletonBookSuccess, SkeletonCancelInfo } from '@/shared/components/common/Skeletons';
import { SkeletonComplexHeader } from '@/features/public-booking/components/SkeletonComplexHeader';
import { SkeletonSlotGrid } from '@/features/public-booking/components/SkeletonSlotGrid';
import { lazyPage } from './routeHelpers';

export const publicRoutes: RouteObject[] = [
  {
    element: <PublicLayout />,
    children: [
      {
        path: '/:slug',
        element: lazyPage(
          () => import('@/pages/public/ComplexPage'),
          <div className="animate-fade-in w-full space-y-6 sm:space-y-10">
            <SkeletonComplexHeader />
            <SkeletonSlotGrid />
          </div>,
        ),
      },
      {
        // Not a page anyone navigates to — MercadoPago's failure back_url.
        // See BookPage for why it must not be removed.
        path: '/:slug/book',
        element: lazyPage(() => import('@/pages/public/BookPage')),
      },
      {
        path: '/:slug/book/confirm',
        element: lazyPage(() => import('@/pages/public/BookConfirmPage'), <SkeletonBookConfirm />),
      },
      {
        path: '/:slug/book/success',
        element: lazyPage(() => import('@/pages/public/BookSuccessPage'), <SkeletonBookSuccess />),
      },
      {
        path: '/:slug/book/cancel',
        element: lazyPage(() => import('@/pages/public/BookCancelPage'), <SkeletonCancelInfo />),
      },
    ],
  },
];
