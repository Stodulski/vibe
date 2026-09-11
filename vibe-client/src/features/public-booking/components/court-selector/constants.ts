import { Sun, Sunset, Moon } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { DurationMinutes } from '@/shared/types/api.types';
import type { TimeGroup } from './slotMath';

const t = ES_AR;

export const COURT_TYPE_LABELS: Record<string, string> = {
  indoor: t.publicBooking.indoor,
  outdoor: t.publicBooking.outdoor,
  semi_covered: t.publicBooking.semi_covered,
};

export const DURATION_OPTIONS: DurationMinutes[] = [60, 90, 120];

export const TIME_GROUPS: {
  key: TimeGroup;
  label: string;
  icon: typeof Sun;
  from: number;
  to: number;
}[] = [
  { key: 'morning', label: t.publicBooking.morning, icon: Sun, from: 0, to: 12 },
  { key: 'afternoon', label: t.publicBooking.afternoon, icon: Sunset, from: 12, to: 18 },
  { key: 'evening', label: t.publicBooking.evening, icon: Moon, from: 18, to: 24 },
];
