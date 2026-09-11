import { MPConnectCard } from '@/features/complex';
import type { Complex } from '@/shared/types/api.types';

/**
 * How money is taken: what is charged up front, and through what.
 *
 * These were two tabs — one holding two numbers, the other holding one button.
 * They are one subject: the deposit percentage is the figure MercadoPago
 * actually charges, and the cancellation window is what decides whether it is
 * refunded. Reading them apart meant setting a deposit without seeing whether
 * anything could collect it.
 *
 * The old tab was also called "Reservas", the same word as the bookings page
 * in the main nav. Two different things under one name.
 */
export function SettingsBillingTab({ complex }: { complex: Complex }) {
  return <MPConnectCard complexId={complex.id} province={complex.province} />;
}
