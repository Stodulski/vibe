import { CheckCircle2, AlertTriangle } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { StatusHero } from '@/shared/components/common/StatusHero';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import type { RefundEnvelope, RefundStatus } from '@/shared/types/api.types';

const t = ES_AR;

interface CancelledResultStateProps {
  refund: RefundEnvelope;
  onBack: () => void;
}

// The complex has to hand this one back by hand — every other outcome either
// already moved automatically or never needed to move at all. `refund.status`
// is the server's own state machine (`data.RefundResult`); this is the one
// place it is used, and only to pick a tone, never to write copy.
const NEEDS_HUMAN_ACTION: ReadonlySet<RefundStatus> = new Set(['manual']);

export function CancelledResultState({ refund, onBack }: CancelledResultStateProps) {
  const needsAction = NEEDS_HUMAN_ACTION.has(refund.status);

  return (
    <StatusHero
      icon={needsAction ? AlertTriangle : CheckCircle2}
      tone={needsAction ? 'warning' : 'success'}
      size="large"
      title={t.publicBooking.cancelBookingSuccess}
      // `refund.message` is the server's own Spanish sentence for this exact
      // outcome (`internal/bookings/cancel.go`'s `refundMessages`) — render it
      // verbatim rather than re-deriving copy from `refund.status`, which is a
      // six-valued machine the client does not own. The fallback only guards
      // against an unexpected empty string; it is not a seventh message.
      description={refund.message || t.publicBooking.cancelledSuccessfully}
      descriptionClassName="max-w-sm"
    >
      {refund.amount !== undefined && (
        <p className="text-text-primary text-base font-semibold">{formatPrice(refund.amount)}</p>
      )}
      {refund.manual_amount !== undefined && (
        <div className="mt-2 max-w-sm text-left">
          <p className="text-text-secondary text-sm">{refund.manual_message}</p>
          <p className="text-text-primary text-base font-semibold">{formatPrice(refund.manual_amount)}</p>
        </div>
      )}
      <Button variant="outline" className="mt-4 min-h-12 rounded-xl" onClick={onBack}>
        {t.publicBooking.makeAnother}
      </Button>
    </StatusHero>
  );
}
