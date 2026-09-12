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
            classNames: {
              toast: 'toast-base',
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
