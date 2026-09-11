import { Suspense } from 'react';
import type { RouteObject } from 'react-router-dom';
import { ErrorBoundary } from '@/shared/components/common/ErrorBoundary';
import { GuestRoute } from './GuestRoute';
import { lazyRetry } from './routeHelpers';
import { PublicPageLoader } from './loaders';

// Standalone auth pages (full-screen loader while auth check runs)
const LoginPage = lazyRetry(() => import('@/pages/auth/LoginPage'));
const RegisterPage = lazyRetry(() => import('@/pages/auth/RegisterPage'));
const GoogleCompletePage = lazyRetry(() => import('@/pages/auth/GoogleCompletePage'));
const VerifyEmailSentPage = lazyRetry(() => import('@/pages/auth/VerifyEmailSentPage'));
const VerifyEmailPage = lazyRetry(() => import('@/pages/auth/VerifyEmailPage'));
const ForgotPasswordPage = lazyRetry(() => import('@/pages/auth/ForgotPasswordPage'));
const ResetPasswordPage = lazyRetry(() => import('@/pages/auth/ResetPasswordPage'));

export const authRoutes: RouteObject[] = [
  {
    path: '/login',
    element: (
      <GuestRoute>
        <ErrorBoundary>
          <Suspense fallback={<PublicPageLoader />}>
            <LoginPage />
          </Suspense>
        </ErrorBoundary>
      </GuestRoute>
    ),
  },
  {
    path: '/register',
    element: (
      <GuestRoute>
        <ErrorBoundary>
          <Suspense fallback={<PublicPageLoader />}>
            <RegisterPage />
          </Suspense>
        </ErrorBoundary>
      </GuestRoute>
    ),
  },
  {
    path: '/register/google',
    element: (
      <GuestRoute>
        <ErrorBoundary>
          <Suspense fallback={<PublicPageLoader />}>
            <GoogleCompletePage />
          </Suspense>
        </ErrorBoundary>
      </GuestRoute>
    ),
  },
  {
    path: '/verify-email-sent',
    element: (
      <ErrorBoundary>
        <Suspense fallback={<PublicPageLoader />}>
          <VerifyEmailSentPage />
        </Suspense>
      </ErrorBoundary>
    ),
  },
  {
    path: '/verify-email',
    element: (
      <ErrorBoundary>
        <Suspense fallback={<PublicPageLoader />}>
          <VerifyEmailPage />
        </Suspense>
      </ErrorBoundary>
    ),
  },
  {
    path: '/forgot-password',
    element: (
      <ErrorBoundary>
        <Suspense fallback={<PublicPageLoader />}>
          <ForgotPasswordPage />
        </Suspense>
      </ErrorBoundary>
    ),
  },
  {
    path: '/reset-password',
    element: (
      <ErrorBoundary>
        <Suspense fallback={<PublicPageLoader />}>
          <ResetPasswordPage />
        </Suspense>
      </ErrorBoundary>
    ),
  },
];
