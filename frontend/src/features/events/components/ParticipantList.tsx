import { ES_AR } from '@/shared/i18n/es_AR';
import { cn, formatPrice } from '@/shared/lib/utils';
import { participantStatusKey, type ParticipantStatusKey } from '../lib/participantStatus';
import type { OpenPlayParticipant } from '@/shared/types/api.types';

const t = ES_AR.events;

// Same colours as the booking payment statuses: amber is money still moving,
// green is paid, red is cancelled.
const STATUS_STYLES: Record<ParticipantStatusKey, string> = {
  held: 'text-warning-text',
  confirmed: 'text-success-text',
  cancelledUnpaid: 'text-error-text',
  refunded: 'text-text-secondary',
  refundPending: 'text-warning-text',
  partialRefund: 'text-warning-text',
};

function moneyLine(participant: OpenPlayParticipant): string | null {
  const payment = participant.payment;
  if (!payment) return null;
  if (payment.refund_amount > 0) return t.participant.refundedAmount(formatPrice(payment.refund_amount));
  return participant.status === 'confirmed' ? t.participant.paid(formatPrice(payment.amount)) : null;
}

function cancelledBy(participant: OpenPlayParticipant): string | null {
  if (participant.status !== 'cancelled') return null;
  if (participant.cancelled_by === 'owner') return t.participant.cancelledByOwner;
  if (participant.cancelled_by === 'participant') return t.participant.cancelledByParticipant;
  return null;
}

function ParticipantRow({ participant }: { participant: OpenPlayParticipant }) {
  const key = participantStatusKey(participant);
  const detail = [moneyLine(participant), cancelledBy(participant)].filter(Boolean).join(' · ');

  return (
    <li className="border-border-subtle flex items-start justify-between gap-3 border-b py-3 last:border-b-0">
      <div className="min-w-0">
        <p className="text-text-primary text-sm font-medium break-words">
          {participant.first_name} {participant.last_name}
        </p>
        <a
          href={`tel:${participant.phone}`}
          aria-label={`${t.participant.call} ${participant.first_name} ${participant.last_name}`}
          className="text-text-tertiary score-text text-xs break-all hover:underline"
        >
          {participant.phone}
        </a>
        {detail && <p className="text-text-tertiary mt-0.5 text-xs">{detail}</p>}
      </div>
      <span className={cn('shrink-0 text-xs font-medium', STATUS_STYLES[key])}>{t.participant[key]}</span>
    </li>
  );
}

/** Everyone who held a spot, each with their own payment state. */
export function ParticipantList({ participants }: { participants: OpenPlayParticipant[] }) {
  if (participants.length === 0) {
    return <p className="text-text-tertiary text-sm">{t.detail.noParticipants}</p>;
  }
  return (
    <ul aria-label={t.detail.participants}>
      {participants.map((participant) => (
        <ParticipantRow key={participant.id} participant={participant} />
      ))}
    </ul>
  );
}
