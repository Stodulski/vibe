export const queryKeys = {
  auth: {
    me: ['auth', 'me'] as const,
  },
  complexes: {
    all: ['complexes'] as const,
    detail: (id: string) => ['complexes', id] as const,
    bySlug: (slug: string) => ['complexes', 'slug', slug] as const,
    schedules: (id: string) => ['complexes', id, 'schedules'] as const,
    mpStatus: (id: string) => ['complexes', id, 'mp-status'] as const,
  },
  courts: {
    byComplex: (complexId: string) => ['courts', complexId] as const,
    detail: (id: string) => ['courts', id] as const,
    blockedSlotsBase: (complexId: string) => ['courts', 'blockedSlots', complexId] as const,
    blockedSlots: (complexId: string, dateFrom: string, dateTo: string) =>
      ['courts', 'blockedSlots', complexId, dateFrom, dateTo] as const,
  },
  bookings: {
    byComplex: (complexId: string) => ['bookings', complexId] as const,
    byDate: (complexId: string, date: string) => ['bookings', complexId, date] as const,
    // 'detail' segment keeps this from colliding with `byComplex(id)`, which
    // shares the same `['bookings', <string>]` shape — without it,
    // invalidating one prefix-matches and clears the other's cache entries too.
    detail: (id: string) => ['bookings', 'detail', id] as const,
  },
  clients: {
    byComplex: (complexId: string) => ['clients', complexId] as const,
    detail: (complexId: string, clientId: string) => ['clients', complexId, clientId] as const,
  },
  availability: {
    // `duration` is appended after `date` so a duration change (60/90/120)
    // is its own cache entry — otherwise the picker would serve stale slots
    // for the previous duration.
    bySlugAndDate: (slug: string, date: string, duration: number) => ['availability', slug, date, duration] as const,
  },
  publicComplex: {
    bySlug: (slug: string) => ['publicComplex', slug] as const,
  },
  mpFees: {
    // Not complex-scoped: one shared static file on the landing, matched
    // against a province client-side.
    costs: ['mpFees', 'costs'] as const,
  },
  bookingStatus: {
    byId: (id: string) => ['bookingStatus', id] as const,
  },
  dashboard: {
    stats: (complexId: string) => ['dashboard', 'stats', complexId] as const,
    revenue: (complexId: string, period: string) => ['dashboard', 'revenue', complexId, period] as const,
    occupancy: (complexId: string) => ['dashboard', 'occupancy', complexId] as const,
    clients: (complexId: string) => ['dashboard', 'clients', complexId] as const,
  },
  admin: {
    stats: ['admin', 'stats'] as const,
    usersBase: ['admin', 'users'] as const,
    users: (params?: { search?: string; role?: string }) => ['admin', 'users', params] as const,
    userDetail: (id: string) => ['admin', 'users', id] as const,
    complexes: (params?: { search?: string }) => ['admin', 'complexes', params] as const,
    complexDetail: (id: string) => ['admin', 'complexes', id] as const,
  },
};
