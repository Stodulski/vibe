import { BOOKING_MESSAGES } from '../../messages';
import type { DurationMinutes } from '../../api/types';

const t = BOOKING_MESSAGES;

export const COURT_TYPE_LABELS: Record<string, string> = {
  indoor: t.publicBooking.indoor,
  outdoor: t.publicBooking.outdoor,
  semi_covered: t.publicBooking.semi_covered,
};

export const DURATION_OPTIONS: DurationMinutes[] = [60, 90, 120];
