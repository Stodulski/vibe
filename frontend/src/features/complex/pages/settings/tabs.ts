import { Building2, Camera, Clock, CreditCard } from 'lucide-react';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * The three sections of a complex's settings.
 *
 * It was five. "Imágenes" held two controls, "Reservas" held two numbers and
 * "Pagos" held one button, and at 375px the strip of five measured 532px in a
 * 341px window — "Reservas" and "Pagos" sat entirely off-screen with nothing
 * showing they were there. Three labels fit, so the fix is fewer sections
 * rather than a scroll cue for sections nobody could see.
 *
 * They merged along the seams that were already there: logo and cover are the
 * same subject as name, URL and description (what the public sees), and the
 * deposit percentage is the figure MercadoPago actually charges, so the two
 * money screens are one.
 *
 * Labels only. These carried a `description` too, rendered by the nav AND
 * repeated by the panel heading word for word — and the nav was too narrow to
 * finish it ("Logo y portada de tu compl..."). Both are gone.
 */
export const TABS = [
  {
    value: 'general',
    label: t.complex.general,
    icon: Building2,
  },
  {
    value: 'schedules',
    label: t.complex.schedules,
    icon: Clock,
  },
  {
    value: 'billing',
    label: t.complex.billing,
    icon: CreditCard,
  },
] as const;

/**
 * Offered only while `GET complexes/{id}/instagram` answers `enabled: true`:
 * the feature stays off in production until Meta approves the app, and a tab
 * whose only button answers "not available" is worse than no tab.
 */
export const INSTAGRAM_TAB = {
  value: 'instagram',
  label: t.instagram.tab,
  icon: Camera,
} as const;

export type TabValue = (typeof TABS)[number]['value'] | typeof INSTAGRAM_TAB.value;
