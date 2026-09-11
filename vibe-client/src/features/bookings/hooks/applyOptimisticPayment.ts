import type { Booking } from '@/shared/types/api.types';

/**
 * Computes the optimistic post-payment state of a booking for the mutation
 * cache update. Mirrors the server's cumulative-deposit rule: a new payment
 * adds to any existing deposit, and the booking is `fully_paid` once the
 * running total reaches (or exceeds) the booking price.
 *
 * It writes `collection_status` only. The refund axis is a separate field and
 * confirming a payment says nothing about it.
 */
export function applyOptimisticPayment(booking: Booking, amount: number): Booking {
  const totalPaid = booking.collection_status === 'deposit_paid' ? booking.deposit_amount + amount : amount;
  const fullyPaid = totalPaid >= booking.price;
  return {
    ...booking,
    collection_status: fullyPaid ? 'fully_paid' : 'deposit_paid',
    deposit_amount: fullyPaid ? booking.deposit_amount : totalPaid,
  };
}
