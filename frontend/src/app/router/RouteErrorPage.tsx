import { useEffect } from 'react';
import { useRouteError, isRouteErrorResponse } from 'react-router-dom';
import * as Sentry from '@sentry/react';
import { AlertTriangle } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * `errorElement` for the router's root route.
 *
 * `routeHelpers.tsx`'s `<ErrorBoundary>` only wraps each lazy page component,
 * so a render error thrown above it — `DashboardLayout`/`AdminLayout`/
 * `PublicLayout`, `ProtectedRoute`/`GuestRoute`, `RootRedirect`,
 * `NotFoundPage`, or `ScrollToTop` — had no boundary to land on and left a
 * blank screen. This sits on the router's root route (see `router.tsx`) so
 * any of those bubble here instead, mirroring `ErrorBoundary`'s fallback UI.
 */
export function RouteErrorPage() {
  const error = useRouteError();

  useEffect(() => {
    const reported = isRouteErrorResponse(error)
      ? new Error(`Route error response: ${String(error.status)} ${error.statusText}`)
      : error instanceof Error
        ? error
        : new Error(String(error));
    Sentry.captureException(reported);
  }, [error]);

  return (
    <div
      className="animate-fade-in flex min-h-[400px] items-center justify-center p-4 sm:p-8"
      role="alert"
      aria-live="assertive"
    >
      <div className="border-error-border/30 bg-bg-subtle w-full max-w-sm rounded-2xl border p-6 text-center shadow-lg shadow-black/10 sm:p-10">
        <div className="bg-error-bg ring-error-border/10 mx-auto mb-6 flex size-14 items-center justify-center rounded-2xl ring-4">
          <AlertTriangle className="text-error-icon size-7" aria-hidden="true" />
        </div>
        <h2 className="text-text-primary text-lg font-bold">{t.layout.errorTitle}</h2>
        <p className="text-text-secondary mt-2 text-sm leading-relaxed">{t.layout.errorDescription}</p>
        <div className="mt-8 flex flex-col-reverse items-center justify-center gap-3 sm:flex-row sm:gap-4">
          <Button
            variant="outline"
            onClick={() => {
              window.location.reload();
            }}
            className="w-full rounded-xl sm:w-auto"
          >
            {t.layout.retry}
          </Button>
          <Button
            onClick={() => {
              window.location.href = '/';
            }}
            className="w-full rounded-xl sm:w-auto"
          >
            {t.layout.backHome}
          </Button>
        </div>
      </div>
    </div>
  );
}
