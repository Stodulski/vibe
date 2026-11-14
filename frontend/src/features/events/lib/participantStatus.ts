import type { OpenPlayParticipant } from '@/shared/types/api.types';

export type ParticipantStatusKey =
  'held' | 'confirmed' | 'cancelledUnpaid' | 'refunded' | 'refundPending' | 'partialRefund';

/**
 * The one state a participant row shows, from the spot's status and the
 * payment behind it: a spot still being paid, a paid one, or a cancelled one
 * and what became of its money. The refund states come off the payment's own
 * status, which is where the server keeps them.
 */
export function participantStatusKey(participant: OpenPlayParticipant): ParticipantStatusKey {
  if (participant.status === 'held') return 'held';
  if (participant.status === 'confirmed') return 'confirmed';
  switch (participant.payment?.status) {
    case 'refunded':
      return 'refunded';
    case 'refund_pending':
      return 'refundPending';
    case 'partial_refund':
      return 'partialRefund';
    default:
      return 'cancelledUnpaid';
  }
}

/** How many spots are paid: the ones cancelling the event refunds. */
export function paidCount(participants: OpenPlayParticipant[]): number {
  return participants.filter((p) => p.status === 'confirmed').length;
}
