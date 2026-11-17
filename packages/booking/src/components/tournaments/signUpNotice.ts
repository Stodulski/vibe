import { Ban, Info, Lock, type LucideIcon } from 'lucide-react';
import { BOOKING_MESSAGES } from '../../messages';
import type { PublicTournamentCategory } from '../../api/types';

const t = BOOKING_MESSAGES.tournaments;

/** Why a category takes no pairs, with a neutral icon: none of these is a success. */
export function noticeFor(category: PublicTournamentCategory): { icon: LucideIcon; text: string } {
  if (category.status === 'cancelled') return { icon: Ban, text: t.categoryCancelled };
  if (category.status === 'closed' || category.draw_generated) return { icon: Lock, text: t.signUpClosedCategory };
  return { icon: Info, text: t.categoryFull };
}
