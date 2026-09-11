import type { DurationMinutes } from '@/shared/types/api.types';

/**
 * What a click on a free slot hands the create-booking form.
 *
 * The duration is decided in the grid, not left to the form's default: the
 * slot's popover only offers durations that fit before whatever comes next on
 * that court, so the choice already carries that constraint with it.
 */
export interface CreateBookingPrefill {
  court_id: string;
  date: string;
  start_time: string;
  duration_minutes: DurationMinutes;
}
