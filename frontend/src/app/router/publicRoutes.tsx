import type { RouteObject } from 'react-router-dom';
import { SkeletonBookConfirm, SkeletonBookSuccess, SkeletonCancelInfo } from '@/shared/components/common/Skeletons';
import { SkeletonComplexHeader } from '@/features/public-booking/components/SkeletonComplexHeader';
import { SkeletonSlotGrid } from '@/features/public-booking/components/SkeletonSlotGrid';
import { lazyPage, lazyShell } from './routeHelpers';
import { PublicPageLoader } from './loaders';
import { LegacyComplexRedirect } from './LegacyComplexRedirect';

// Every booking route loads from `publicBookingPages`, which renders the booking
// entries. The entries import all the booking pages, so the booking pages share
// one lazy chunk.
export const publicRoutes: RouteObject[] = [
  {
    element: lazyShell(
      () => import('@/shared/components/layout/PublicLayout').then((m) => ({ default: m.PublicLayout })),
      <PublicPageLoader />,
    ),
    children: [
      {
        path: '/c/:slug',
        element: lazyPage(
          () => import('./publicBookingPages').then((m) => ({ default: m.ComplexPageRoute })),
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
        element: lazyPage(() => import('./publicBookingPages').then((m) => ({ default: m.BookPageRoute }))),
      },
      {
        path: '/c/:slug/book/confirm',
        element: lazyPage(
          () => import('./publicBookingPages').then((m) => ({ default: m.BookConfirmRoute })),
          <SkeletonBookConfirm />,
        ),
      },
      {
        path: '/c/:slug/book/success',
        element: lazyPage(
          () => import('./publicBookingPages').then((m) => ({ default: m.BookSuccessRoute })),
          <SkeletonBookSuccess />,
        ),
      },
      {
        path: '/c/:slug/book/cancel',
        element: lazyPage(
          () => import('./publicBookingPages').then((m) => ({ default: m.BookCancelRoute })),
          <SkeletonCancelInfo />,
        ),
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
