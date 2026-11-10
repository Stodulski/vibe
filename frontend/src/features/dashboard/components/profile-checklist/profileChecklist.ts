import type { Complex, CourtWithPrices } from '@/shared/types/api.types';

// Shared with the settings field, so the hint there and the rule here agree.
export { MIN_DESCRIPTION_LENGTH } from '@/features/complex';
import { MIN_DESCRIPTION_LENGTH } from '@/features/complex';

export type ProfileItemId =
  'cover' | 'logo' | 'description' | 'amenities' | 'location' | 'phone' | 'pricedCourt' | 'mercadoPago';

export interface ProfileChecklistItem {
  id: ProfileItemId;
  done: boolean;
}

interface ProfileChecklistInput {
  complex: Complex;
  courts: CourtWithPrices[];
}

function isFilled(value: string | null | undefined): boolean {
  return (value ?? '').trim() !== '';
}

function hasPricedActiveCourt(courts: CourtWithPrices[]): boolean {
  return courts.some((court) => court.is_active && court.prices.some((price) => price.price > 0));
}

/**
 * The public-profile checklist for one complex, in the order the owner fills
 * it. Pure: everything it knows comes from the complex and its courts, so the rules can be tested without rendering anything.
 */
export function buildProfileChecklist({ complex, courts }: ProfileChecklistInput): ProfileChecklistItem[] {
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
    // No "hours" item: the seeded week (08:00-23:00) is a valid answer, and
    // nothing records that the owner reviewed it, so the item could never be
    // ticked by an owner whose real hours are the default ones.
    { id: 'phone', done: isFilled(complex.phone) },
    { id: 'pricedCourt', done: hasPricedActiveCourt(courts) },
    // The same flag the settings billing card reads as "conectado" (mp/status `connected`).
    { id: 'mercadoPago', done: complex.payments_enabled },
  ];
}
