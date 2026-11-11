import { ES_AR } from '@/shared/i18n/es_AR';
import type { BalanceDue } from '@/shared/types/api.types';

const t = ES_AR;

/**
 * A booking's remaining balance on the "Vender" cart. There is no quantity:
 * the cashier collects the whole remaining amount or nothing. `amount` is the
 * remaining balance at the moment it was tapped; the checkout sends it, and the
 * server refuses it (409) if the booking's balance has moved since.
 */
export interface BalanceLine {
  bookingId: string;
  courtName: string;
  startTime: string;
  clientName: string;
  amount: number;
}

/**
 * One booking balance per sale (owner decision, 2026-11-11). A sale has one
 * payment method, and two bookings are almost always two clients paying
 * apart, so mixing them in one sale makes it unclear who paid what. The
 * server would accept up to 20; the counter allows one.
 */
export const MAX_BALANCE_LINES = 1;

/**
 * Tapping a booking tile: puts its remaining balance on the cart. The same
 * booking again returns the same array so callers can tell nothing changed;
 * a different booking replaces the one already there.
 */
export function addBalanceLine(lines: BalanceLine[], booking: BalanceDue): BalanceLine[] {
  if (lines.some((line) => line.bookingId === booking.booking_id)) return lines;
  return [
    {
      bookingId: booking.booking_id,
      courtName: booking.court_name,
      startTime: booking.start_time,
      clientName: booking.client_name,
      amount: booking.remaining,
    },
  ];
}

export function removeBalanceLine(lines: BalanceLine[], bookingId: string): BalanceLine[] {
  return lines.filter((line) => line.bookingId !== bookingId);
}

export function balancesTotal(lines: BalanceLine[]): number {
  return lines.reduce((sum, line) => sum + line.amount, 0);
}

/**
 * Keeps only the lines whose booking still owes exactly the amount in the cart
 * (`bookings` is today's list). A booking that was paid, cancelled, marked no
 * show, or fell out of its payment window is gone from the list; a booking
 * whose balance moved no longer matches. Either way the line is dropped, so
 * the cashier never charges an amount the screen did not show.
 */
export function reconcileBalances(
  lines: BalanceLine[],
  bookings: BalanceDue[],
): { lines: BalanceLine[]; dropped: boolean } {
  const owed = new Map(bookings.map((booking) => [booking.booking_id, booking.remaining]));
  const kept = lines.filter((line) => owed.get(line.bookingId) === line.amount);
  return { lines: kept, dropped: kept.length < lines.length };
}

/** The cart and sales-row label for a balance: "Saldo turno · Cancha 2 19:00 · Juan Pérez". */
export function bookingBalanceLabel(courtName: string, startTime: string, clientName: string): string {
  return `${t.cash.balanceLinePrefix} · ${courtName} ${startTime} · ${clientName}`;
}
