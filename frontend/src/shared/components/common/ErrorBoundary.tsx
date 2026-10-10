import type { ComponentProps } from 'react';
import { ErrorBoundary as UiErrorBoundary } from '@vibe/ui/components/common/ErrorBoundary.tsx';
import { captureException } from '@/shared/lib/observability';

/**
 * The shared boundary with the app's error reporting: every caught error still
 * goes to Sentry, tagged with the route. The package itself reports nothing;
 * a caller's `onError` runs after the report.
 */
export function ErrorBoundary({ onError, ...props }: ComponentProps<typeof UiErrorBoundary>) {
  return (
    <UiErrorBoundary
      {...props}
      onError={(error, info) => {
        captureException(error, {
          tags: { route: window.location.pathname },
          extra: { componentStack: info.componentStack },
        });
        onError?.(error, info);
      }}
    />
  );
}
