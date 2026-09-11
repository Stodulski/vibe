import { timeToMinutes, generateTimeSlots } from '@/shared/lib/time';

// Re-exported so existing call sites (`useBlockSlotForm.ts`, this file's own
// test) keep importing from the feature-local module; the implementation now
// lives in `src/shared/lib/time.ts` (shared with `src/features/bookings`)
// to avoid duplicating the "HH:MM" slot-generation/conversion logic across
// features.
export { timeToMinutes, generateTimeSlots };

export const ALL_TIME_SLOTS = generateTimeSlots();
