import { useMemo, type ReactNode } from 'react';
import { QueryClientProvider, type QueryClient } from '@tanstack/react-query';
import { Toaster } from 'sonner';
import { BookingConfigContext, type BookingConfig } from './config';
import { PublicBookingApiContext } from './api/context';
import { createBookingClient } from './api/client';
import { createPublicBookingApi } from './api/public-booking.api';
import { getBookingQueryClient } from './lib/bookingQueryClient';

export interface BookingRootProps {
  config: BookingConfig;
  /**
   * Mount a sonner Toaster. Pass `false` where the host app already mounts one,
   * or every toast is rendered twice.
   */
  toaster?: boolean;
  /** Test seam: a fresh client per test. Production uses the browser singleton. */
  queryClient?: QueryClient;
  children: ReactNode;
}

const TOAST_CLASS_NAMES = {
  toast: 'toast-base',
  error: 'toast-error',
  success: 'toast-success',
  warning: 'toast-warning',
};

/**
 * Provides everything one booking page needs: the config, the booking endpoints,
 * the query cache and, optionally, the toasts. Each page entry renders one of these.
 */
export function BookingRoot({ config, toaster = true, queryClient, children }: BookingRootProps) {
  const client = queryClient ?? getBookingQueryClient(config.reportError);
  const api = useMemo(() => createPublicBookingApi(createBookingClient(config.apiBaseUrl)), [config.apiBaseUrl]);

  return (
    <BookingConfigContext.Provider value={config}>
      <PublicBookingApiContext.Provider value={api}>
        <QueryClientProvider client={client}>
          {children}
          {toaster && <Toaster position="bottom-right" richColors toastOptions={{ classNames: TOAST_CLASS_NAMES }} />}
        </QueryClientProvider>
      </PublicBookingApiContext.Provider>
    </BookingConfigContext.Provider>
  );
}
