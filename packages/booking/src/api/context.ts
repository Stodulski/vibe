import { createContext, useContext } from 'react';
import type { PublicBookingApi } from './public-booking.api';

export const PublicBookingApiContext = createContext<PublicBookingApi | null>(null);

/** The booking endpoints of the nearest `BookingRoot`. Throws outside one. */
export function usePublicBookingApi(): PublicBookingApi {
  const api = useContext(PublicBookingApiContext);
  if (!api) {
    throw new Error('usePublicBookingApi must be used inside <BookingRoot>');
  }
  return api;
}
