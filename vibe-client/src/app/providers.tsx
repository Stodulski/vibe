import type { ReactNode } from 'react';
import { QueryClientProvider } from '@tanstack/react-query';
import { TooltipProvider } from '@/shared/components/ui/tooltip';
import { Toaster } from 'sonner';
import { queryClient } from '@/shared/lib/queryClient';

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
            style: {
              background: 'var(--color-bg-elevated)',
              border: '1px solid var(--color-border-default)',
              color: 'var(--color-text-primary)',
            },
            classNames: {
              error: 'toast-error',
              success: 'toast-success',
              warning: 'toast-warning',
            },
          }}
        />
      </TooltipProvider>
    </QueryClientProvider>
  );
}
