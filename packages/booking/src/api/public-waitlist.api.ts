import type { KyInstance } from 'ky';
import type { PublicWaitlistRequest } from './types';

/** The place waitlist endpoint of the public flow (one event, category or slot), merged into `createPublicBookingApi`. The day waitlist lives in `public-booking.api.ts`. */
export function createPublicWaitlistApi(client: KyInstance) {
  return {
    /**
     * Asks to be emailed when a place frees: one person in open play, one pair in a
     * tournament category, or a taken court slot. Idempotent per email and place.
     */
    joinPlaceWaitlist: (slug: string, data: PublicWaitlistRequest) =>
      client.post(`public/complexes/${encodeURIComponent(slug)}/waitlist/places`, { json: data }).then(() => undefined),
  };
}
