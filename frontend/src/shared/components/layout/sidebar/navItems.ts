import { LayoutDashboard, CalendarDays, Wallet, Trophy, Users, FileBarChart, Settings } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export const mainNavItems = [
  { to: '/dashboard', label: t.navigation.dashboard, icon: LayoutDashboard },
  { to: '/bookings', label: t.navigation.bookings, icon: CalendarDays },
  { to: '/cash', label: t.navigation.cash, icon: Wallet },
  { to: '/courts', label: t.navigation.courts, icon: Trophy },
  { to: '/clients', label: t.navigation.clients, icon: Users },
  { to: '/reports', label: t.navigation.reports, icon: FileBarChart },
] as const;

export const secondaryNavItems = [{ to: '/settings', label: t.navigation.settings, icon: Settings }] as const;
