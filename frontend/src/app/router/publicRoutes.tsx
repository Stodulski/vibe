import type { RouteObject } from 'react-router-dom';
import { SkeletonBookConfirm, SkeletonBookSuccess, SkeletonCancelInfo } from '@/shared/components/common/Skeletons';
import { SkeletonComplexHeader } from '@/features/public-booking/components/SkeletonComplexHeader';
import { SkeletonSlotGrid } from '@/features/public-booking/components/SkeletonSlotGrid';
import { lazyPage, lazyShell } from './routeHelpers';
import { PublicPageLoader } from './loaders';
import { LegacyComplexRedirect } from './LegacyComplexRedirect';
import { PublicBookingRoot } from './PublicBookingRoot';

export const publicRoutes: RouteObject[] = [
  {
    element: lazyShell(
      () => import('@/shared/components/layout/PublicLayout').then((m) => ({ default: m.PublicLayout })),
      <PublicPageLoader />,
    ),
    children: [
      {
        // The booking providers wrap only the pages, not the shell, so the
        // header and footer keep the app's own providers.
        element: <PublicBookingRoot />,
        children: [
          {
            path: '/c/:slug',
            element: lazyPage(
              () => import('@/features/public-booking/pages/ComplexPage'),
              <div className="animate-fade-in w-full space-y-6 sm:space-y-10">
                <SkeletonComplexHeader />
                <SkeletonSlotGrid />
              </div>,
            ),
          },
          {
            // Not a page anyone navigates to — MercadoPago's failure back_url.
            // See BookPage for why it must not be removed.
            path: '/c/:slug/book',
            element: lazyPage(() => import('@/features/public-booking/pages/BookPage')),
          },
          {
            path: '/c/:slug/book/confirm',
            element: lazyPage(() => import('@/features/public-booking/pages/BookConfirmPage'), <SkeletonBookConfirm />),
          },
          {
            path: '/c/:slug/book/success',
            element: lazyPage(() => import('@/features/public-booking/pages/BookSuccessPage'), <SkeletonBookSuccess />),
          },
          {
            path: '/c/:slug/book/cancel',
            element: lazyPage(() => import('@/features/public-booking/pages/BookCancelPage'), <SkeletonCancelInfo />),
          },
        ],
      },
    ],
  },
  // The pre-`/c/` addresses of a complex storefront and its booking steps.
  // Outside the shell on purpose: they only redirect, so they load no layout
  // chunk. Each path is spelled out, so nothing else is caught by them.
  { path: '/:slug', element: <LegacyComplexRedirect /> },
  { path: '/:slug/book', element: <LegacyComplexRedirect /> },
  { path: '/:slug/book/confirm', element: <LegacyComplexRedirect /> },
  { path: '/:slug/book/success', element: <LegacyComplexRedirect /> },
  { path: '/:slug/book/cancel', element: <LegacyComplexRedirect /> },
];
