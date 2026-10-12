import type { Complex, CourtWithPrices, Schedule } from '@/shared/types/api.types';

/**
 * Shortest description that counts as written. A line or two gives search
 * engines too little text to say what the venue is, so anything under one
 * full sentence is still "to do".
 */
export const MIN_DESCRIPTION_LENGTH = 80;

// What `createComplex` seeds for every day of the week (service.go, Create).
const DEFAULT_OPEN_TIME = '08:00';
const DEFAULT_CLOSE_TIME = '23:00';

export type ProfileItemId =
  'cover' | 'logo' | 'description' | 'amenities' | 'location' | 'hours' | 'phone' | 'pricedCourt';

export interface ProfileChecklistItem {
  id: ProfileItemId;
  done: boolean;
}

interface ProfileChecklistInput {
  complex: Complex;
  schedules: Schedule[];
  courts: CourtWithPrices[];
}

function isDefaultDay(schedule: Schedule): boolean {
  return schedule.open_time === DEFAULT_OPEN_TIME && schedule.close_time === DEFAULT_CLOSE_TIME && !schedule.is_closed;
}

/** True only for the seven untouched rows a complex is created with. */
export function isDefaultSchedule(schedules: Schedule[]): boolean {
  return schedules.length === 7 && schedules.every(isDefaultDay);
}

function isFilled(value: string | null | undefined): boolean {
  return (value ?? '').trim() !== '';
}

function hasPricedActiveCourt(courts: CourtWithPrices[]): boolean {
  return courts.some((court) => court.is_active && court.prices.some((price) => price.price > 0));
}

/**
 * The public-profile checklist for one complex, in the order the owner fills
 * it. Pure: everything it knows comes from the complex, its schedules and its
 * courts, so the rules can be tested without rendering anything.
 */
export function buildProfileChecklist({ complex, schedules, courts }: ProfileChecklistInput): ProfileChecklistItem[] {
  const description = (complex.description ?? '').trim();
  return [
    { id: 'cover', done: isFilled(complex.cover_url) },
    { id: 'logo', done: isFilled(complex.logo_url) },
    { id: 'description', done: description.length >= MIN_DESCRIPTION_LENGTH },
    { id: 'amenities', done: complex.amenities.length > 0 },
    {
      id: 'location',
      done: typeof complex.latitude === 'number' && typeof complex.longitude === 'number',
    },
    // No rows at all means nothing was set up, so the hours still need a look.
    { id: 'hours', done: schedules.length > 0 && !isDefaultSchedule(schedules) },
    { id: 'phone', done: isFilled(complex.phone) },
    { id: 'pricedCourt', done: hasPricedActiveCourt(courts) },
  ];
}
