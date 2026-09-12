import { lazy, Suspense, type ReactNode } from 'react';
import { QueryClientProvider } from '@tanstack/react-query';
import { TooltipProvider } from '@/shared/components/ui/tooltip';
import { Toaster } from 'sonner';
import { queryClient } from '@/shared/lib/queryClient';

/**
 * The React Query devtools panel, in development only.
 *
 * The `import.meta.env.DEV` guard is on the *import*, not on the render:
 * Vite substitutes `false` there for a production build, so the whole ternary
 * — and with it the dynamic `import()` — is dropped before bundling and the
 * devtools never become a chunk anyone can download. Guarding only the JSX
 * would keep the import alive and ship the panel to every visitor.
 */
const ReactQueryDevtools = import.meta.env.DEV
  ? lazy(() => import('@tanstack/react-query-devtools').then((m) => ({ default: m.ReactQueryDevtools })))
  : null;

interface ProvidersProps {
  children: ReactNode;
}

export function Providers({ children }: ProvidersProps) {
  return (
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        {children}
        <Toaster
          position="bottom-right"
          richColors
          toastOptions={{
            classNames: {
              toast: 'toast-base',
              error: 'toast-error',
              success: 'toast-success',
              warning: 'toast-warning',
            },
          }}
        />
      </TooltipProvider>
      {ReactQueryDevtools && (
        <Suspense fallback={null}>
          <ReactQueryDevtools initialIsOpen={false} buttonPosition="bottom-left" />
        </Suspense>
      )}
    </QueryClientProvider>
  );
}
