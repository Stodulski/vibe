import {
  Car,
  DoorOpen,
  ShowerHead,
  Beer,
  Handshake,
  ShoppingBag,
  Wifi,
  Lock,
  GraduationCap,
  Trophy,
  Accessibility,
  Video,
  type LucideIcon,
} from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { Amenity } from '@/shared/types/api.types';

const t = ES_AR;

/**
 * What a venue can offer, in the order it is shown.
 *
 * This list and the server's `knownAmenities` map and the complexes_amenities_known CHECK
 * constraint are the same vocabulary written three times. Adding a value means
 * touching all three, and the server test asserts its two agree — a value
 * missing here just never renders, which is the harmless end of the failure.
 *
 * Ordered roughly by how often someone would look for it rather than
 * alphabetically: parking and changing rooms decide whether a club is usable
 * at all, match recording is a nice-to-have.
 */
export const AMENITIES: { value: Amenity; label: string; icon: LucideIcon }[] = [
  { value: 'parking', label: t.complex.amenities.parking, icon: Car },
  { value: 'changing_rooms', label: t.complex.amenities.changing_rooms, icon: DoorOpen },
  { value: 'showers', label: t.complex.amenities.showers, icon: ShowerHead },
  { value: 'bar', label: t.complex.amenities.bar, icon: Beer },
  { value: 'racket_rental', label: t.complex.amenities.racket_rental, icon: Handshake },
  { value: 'pro_shop', label: t.complex.amenities.pro_shop, icon: ShoppingBag },
  { value: 'wifi', label: t.complex.amenities.wifi, icon: Wifi },
  { value: 'lockers', label: t.complex.amenities.lockers, icon: Lock },
  { value: 'lessons', label: t.complex.amenities.lessons, icon: GraduationCap },
  { value: 'tournaments', label: t.complex.amenities.tournaments, icon: Trophy },
  { value: 'accessible', label: t.complex.amenities.accessible, icon: Accessibility },
  { value: 'match_recording', label: t.complex.amenities.match_recording, icon: Video },
];

/**
 * The listed amenities of a complex, in the canonical order above.
 *
 * The server preserves the order it was sent, which is the order the owner
 * happened to tick the boxes in — fine to store, wrong to render: the same two
 * amenities would sit in different places on two different venues' cards.
 */
export function orderedAmenities(selected: readonly string[] | undefined): typeof AMENITIES {
  // Tolerates undefined for the same reason AmenitiesField does: a response
  // cached before this field existed carries no key.
  const chosen = new Set(selected ?? []);
  return AMENITIES.filter((a) => chosen.has(a.value));
}
