import {
  Accessibility,
  Beer,
  Car,
  Handshake,
  DoorOpen,
  GraduationCap,
  Lock,
  ShoppingBag,
  ShowerHead,
  Trophy,
  Video,
  Wifi,
  type LucideIcon,
} from 'lucide-react';
import { BOOKING_MESSAGES } from '../messages';
import type { Amenity } from '../api/types';

const t = BOOKING_MESSAGES.complex.amenities;

/**
 * What a venue can offer, in the order it is shown. Copied from the app's
 * `shared/lib/amenities.ts`: the same vocabulary, the same labels, and the
 * same icons. A value missing here never renders.
 */
export const AMENITIES: { value: Amenity; label: string; icon: LucideIcon }[] = [
  { value: 'parking', label: t.parking, icon: Car },
  { value: 'changing_rooms', label: t.changing_rooms, icon: DoorOpen },
  { value: 'showers', label: t.showers, icon: ShowerHead },
  { value: 'bar', label: t.bar, icon: Beer },
  { value: 'racket_rental', label: t.racket_rental, icon: Handshake },
  { value: 'pro_shop', label: t.pro_shop, icon: ShoppingBag },
  { value: 'wifi', label: t.wifi, icon: Wifi },
  { value: 'lockers', label: t.lockers, icon: Lock },
  { value: 'lessons', label: t.lessons, icon: GraduationCap },
  { value: 'tournaments', label: t.tournaments, icon: Trophy },
  { value: 'accessible', label: t.accessible, icon: Accessibility },
  { value: 'match_recording', label: t.match_recording, icon: Video },
];

/** The listed amenities of a complex, in the canonical order above. Tolerates `undefined` from an older response. */
export function orderedAmenities(selected: readonly string[] | undefined): typeof AMENITIES {
  const chosen = new Set(selected ?? []);
  return AMENITIES.filter((a) => chosen.has(a.value));
}
