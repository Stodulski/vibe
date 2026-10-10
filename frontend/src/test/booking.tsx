import type { ReactNode } from 'react';
import { render, type RenderOptions } from '@testing-library/react';
import { BookingRoot } from '@/features/public-booking/BookingRoot';
import type { BookingConfig } from '@/features/public-booking/config';
import { createBookingQueryClient } from '@/features/public-booking/lib/bookingQueryClient';

/** Matches the MSW handlers, which answer under `VITE_API_URL` of the vitest config. */
const TEST_BOOKING_CONFIG: BookingConfig = {
  apiBaseUrl: 'http://localhost/api/v1',
  publicSiteUrl: 'http://localhost',
  privacyUrl: 'https://vibe.com.ar/privacidad',
};

interface BookingWrapperOptions {
  config?: BookingConfig;
  queryClient?: ReturnType<typeof createBookingQueryClient>;
}

/**
 * The wrapper for a booking hook or component under `renderHook`/`render`: one
 * `BookingRoot` with a fresh query cache, no Toaster. Create one per test.
 */
export function createBookingWrapper({ config = TEST_BOOKING_CONFIG, queryClient }: BookingWrapperOptions = {}) {
  const client = queryClient ?? createBookingQueryClient();
  return function BookingWrapper({ children }: { children: ReactNode }) {
    return (
      <BookingRoot config={config} toaster={false} queryClient={client}>
        {children}
      </BookingRoot>
    );
  };
}

/**
 * Renders a component under a fresh `BookingRoot`. Pages that read the URL bring
 * their own `MemoryRouter` around the element.
 */
export function renderBooking(ui: ReactNode, options?: Omit<RenderOptions, 'wrapper'> & { config?: BookingConfig }) {
  const { config, ...renderOptions } = options ?? {};
  const wrapper = createBookingWrapper(config === undefined ? {} : { config });
  return render(ui, { wrapper, ...renderOptions });
}

/** Sets the URL the booking pages read their query from, through `history.replaceState`. */
export function setBookingUrl(path: string): void {
  window.history.replaceState(null, '', path);
}
