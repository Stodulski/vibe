import { useEffect } from 'react';
import { RouterProvider } from 'react-router-dom';
import { router } from './router';
import { Providers } from './providers';
import { OfflineBanner } from '@/shared/components/common/OfflineBanner';
import { ErrorBoundary } from '@/shared/components/common/ErrorBoundary';

export function App() {
  useEffect(() => {
    const handler = () => {
      document.documentElement.classList.toggle('tab-hidden', document.hidden);
    };
    document.addEventListener('visibilitychange', handler);
    return () => {
      document.removeEventListener('visibilitychange', handler);
    };
  }, []);

  return (
    <Providers>
      {/*
        Belt-and-suspenders: the router's own `errorElement` (see
        `router.tsx`) is what actually catches render errors inside the
        route tree. This only guards whatever renders outside it.
      */}
      <ErrorBoundary>
        <RouterProvider router={router} />
      </ErrorBoundary>
      <OfflineBanner />
    </Providers>
  );
}
