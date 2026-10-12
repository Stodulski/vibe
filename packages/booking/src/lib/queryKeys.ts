/**
 * The query keys of the booking cache. Only the ones the booking flow reads;
 * the strings match the app's `shared/lib/queryKeys.ts` so a cache seeded by
 * either side has the same key.
 */
export const queryKeys = {
  availability: {
    bySlugAndDate: (slug: string, date: string, duration: number) => ['availability', slug, date, duration] as const,
  },
  nextAvailable: {
    bySlugFromAndDuration: (slug: string, from: string, duration: number | null) =>
      ['nextAvailable', slug, from, duration] as const,
  },
  publicComplex: {
    bySlug: (slug: string) => ['publicComplex', slug] as const,
  },
  cancelInfo: {
    byToken: (token: string) => ['cancel-info', token] as const,
  },
  bookingStatus: {
    byId: (id: string) => ['bookingStatus', id] as const,
  },
} as const;
