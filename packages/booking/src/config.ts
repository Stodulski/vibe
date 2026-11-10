import { createContext, useContext } from 'react';

/**
 * Everything the booking flow needs from the app that mounts it. The package
 * reads no environment variables and no app module: whoever mounts `BookingRoot`
 * passes these values in.
 */
export interface BookingConfig {
  /** Absolute URL of the public API, e.g. `https://api.example/api/v1`. */
  apiBaseUrl: string;
  /** Origin the storefront is served from, used for canonical and OG URLs. */
  publicSiteUrl: string;
  /** Absolute URL of the privacy policy linked from the booking form. */
  privacyUrl: string;
  /** Receives every failed query and mutation. Optional: failures still render inline. */
  reportError?: (error: unknown, context?: Record<string, unknown>) => void;
  /**
   * Cloudflare Turnstile site key. When set, the complex page runs the
   * challenge invisibly and every slot hold (`POST /book/holds`) carries its
   * token; the booking that converts the hold needs none. Unset: no widget, no
   * token, and the API must not be enforcing the challenge.
   */
  turnstileSiteKey?: string | undefined;
}

/** Provided by `BookingRoot`. Kept in a `.ts` file so the `.tsx` files export components only. */
export const BookingConfigContext = createContext<BookingConfig | null>(null);

/** The config of the nearest `BookingRoot`. Throws outside one, so a missing provider fails loudly. */
export function useBookingConfig(): BookingConfig {
  const config = useContext(BookingConfigContext);
  if (!config) {
    throw new Error('useBookingConfig must be used inside <BookingRoot>');
  }
  return config;
}
