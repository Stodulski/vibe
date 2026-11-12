import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export const mainNavItems = [
  { to: '/dashboard', label: t.navigation.dashboard },
  { to: '/bookings', label: t.navigation.bookings },
  { to: '/cash', label: t.navigation.cash },
  { to: '/courts', label: t.navigation.courts },
  { to: '/clients', label: t.navigation.clients },
  { to: '/reports', label: t.navigation.reports },
] as const;

export const secondaryNavItems = [{ to: '/settings', label: t.navigation.settings }] as const;
