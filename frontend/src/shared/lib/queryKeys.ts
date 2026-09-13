export const queryKeys = {
  auth: {
    me: ['auth', 'me'] as const,
    // The token is part of the key so the cache itself dedupes the (single-use)
    // verification call across StrictMode's double mount — see useVerifyEmail.
    verifyEmail: (token: string) => ['auth', 'verify-email', token] as const,
  },
  complexes: {
    all: ['complexes'] as const,
    detail: (id: string) => ['complexes', id] as const,
    bySlug: (slug: string) => ['complexes', 'slug', slug] as const,
    schedules: (id: string) => ['complexes', id, 'schedules'] as const,
    mpStatus: (id: string) => ['complexes', id, 'mp-status'] as const,
    slugAvailable: (slug: string) => ['complexes', 'slug-available', slug] as const,
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
  // Not booking-scoped by id: the cancellation link carries an opaque token and
  // the booking it resolves to is exactly what this query answers.
  cancelInfo: {
    byToken: (token: string) => ['cancel-info', token] as const,
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
  // JOB-06's async export: one entry per job id, so a stale poll for a
  // previous export can never satisfy a new one started right after it.
  reportsExport: {
    status: (complexId: string, exportId: string) => ['reportsExport', complexId, exportId] as const,
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
