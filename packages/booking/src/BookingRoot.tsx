import { useMemo, useState, type ReactNode } from 'react';
import { QueryClientProvider, type QueryClient } from '@tanstack/react-query';
import { Toaster } from 'sonner';
import { BookingConfigContext, type BookingConfig } from './config';
import { PublicBookingApiContext } from './api/context';
import { createBookingClient } from './api/client';
import { createPublicBookingApi } from './api/public-booking.api';
import { getBookingQueryClient } from './lib/bookingQueryClient';
import { InitialSearchContext } from './lib/initialSearch';
import { queryKeys } from './lib/queryKeys';
import type { PublicComplexResponse, PublicEventListResponse, PublicTournamentListResponse } from './api/types';

export interface BookingRootProps {
  config: BookingConfig;
  /**
   * Mount a sonner Toaster. Pass `false` where the host app already mounts one,
   * or every toast is rendered twice.
   */
  toaster?: boolean | undefined;
  /**
   * Seeds the complex query, so a page with the complex already known renders without a fetch.
   * `events` and `tournaments` seed the complex's open play and tournaments the same way, so
   * its "Eventos" list is in the server markup instead of waiting for the scripts; a list the
   * server could not read is left out and fetched in the browser as before.
   */
  initialComplex?:
    | {
        slug: string;
        data: PublicComplexResponse;
        events?: PublicEventListResponse | undefined;
        tournaments?: PublicTournamentListResponse | undefined;
      }
    | undefined;
  /**
   * The page's query string as the server saw it (`?date=...`, with the `?`, or
   * empty). The server render and the first client render read it, so the
   * markup matches. Pass `Astro.url.search` on the server.
   */
  initialSearch?: string | undefined;
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
export function BookingRoot({
  config,
  toaster = true,
  initialComplex,
  initialSearch,
  queryClient,
  children,
}: BookingRootProps) {
  const client = queryClient ?? getBookingQueryClient(config.reportError);
  const api = useMemo(
    () => createPublicBookingApi(createBookingClient(config.apiBaseUrl), config.reportError),
    [config.apiBaseUrl, config.reportError],
  );

  // Seeded once per mount, before any child reads the cache.
  useState(() => {
    if (initialComplex) {
      const { slug, data, events, tournaments } = initialComplex;
      client.setQueryData(queryKeys.publicComplex.bySlug(slug), data);
      // The page is served from a cache for up to minutes, so these lists are shown at once but
      // marked stale (updatedAt 0): the browser still refetches them, and an event created since
      // the page was cached appears without waiting for the cache.
      if (events) client.setQueryData(queryKeys.events.bySlug(slug), events, { updatedAt: 0 });
      if (tournaments) client.setQueryData(queryKeys.tournaments.bySlug(slug), tournaments, { updatedAt: 0 });
    }
    return true;
  });

  return (
    <BookingConfigContext.Provider value={config}>
      <PublicBookingApiContext.Provider value={api}>
        <InitialSearchContext.Provider value={initialSearch}>
          <QueryClientProvider client={client}>
            {children}
            {/* At the top: on a phone a bottom toast sat over "Reservar" and,
                once touched, stayed there (sonner pauses while hovered). */}
            {toaster && <Toaster position="top-center" richColors toastOptions={{ classNames: TOAST_CLASS_NAMES }} />}
          </QueryClientProvider>
        </InitialSearchContext.Provider>
      </PublicBookingApiContext.Provider>
    </BookingConfigContext.Provider>
  );
}
