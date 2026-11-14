import { useState } from 'react';
import { CheckCircle2 } from 'lucide-react';
import { Button, ConfirmDialog, StatusHero, formatDeadline, formatPrice } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { publicComplexPath } from '../../lib/publicPaths';
import type { RefundEnvelope } from '../../api/types';

const t = BOOKING_MESSAGES;

/** The outcome of a cancelled spot: the server's own Spanish sentence for the refund, as it comes. */
export function CancelledResult({ refund, slug, title }: { refund: RefundEnvelope; slug: string; title: string }) {
  return (
    <StatusHero
      icon={CheckCircle2}
      tone={refund.status === 'manual' ? 'warning' : 'success'}
      size="large"
      title={title}
      description={refund.message || t.publicBooking.cancelledSuccessfully}
      descriptionClassName="max-w-sm"
    >
      {refund.amount !== undefined && (
        <p className="text-text-primary text-base font-semibold">{formatPrice(refund.amount)}</p>
      )}
      <Button asChild variant="outline" className="min-h-12">
        <a href={publicComplexPath(slug)}>{t.events.backToComplex}</a>
      </Button>
    </StatusHero>
  );
}

export interface CancelCopy {
  action: string;
  confirmTitle: string;
  confirmRefund: string;
  confirmNoRefund: string;
  /** What leaving costs once the draw exists, when the sign-up has such a state. */
  confirmWalkover?: string;
  /** A line above the button that holds for the walkover case. */
  walkoverNote?: string;
}

interface CancelRegistrationProps {
  withinWindow: boolean;
  deadline: string;
  /** After a tournament draw: leaving forfeits the matches and keeps the payment. */
  walkover?: boolean;
  cancel: {
    mutate: (variables: undefined, options: { onSettled: () => void }) => void;
    isPending: boolean;
  };
  copy: CancelCopy;
}

/** The refund window of the complex policy, the cancel button and its confirmation. */
export function CancelRegistration({ withinWindow, deadline, walkover = false, cancel, copy }: CancelRegistrationProps) {
  const [confirmOpen, setConfirmOpen] = useState(false);
  const afterDraw = walkover && copy.confirmWalkover !== undefined;
  const note = afterDraw
    ? copy.walkoverNote
    : withinWindow
      ? `${t.events.refundUntilPrefix} ${formatDeadline(deadline)}.`
      : t.events.refundWindowPassed;
  const description = afterDraw ? copy.confirmWalkover : withinWindow ? copy.confirmRefund : copy.confirmNoRefund;

  return (
    <div className="flex flex-col gap-3">
      {note && <p className="text-text-secondary text-center text-sm">{note}</p>}
      <Button
        variant="destructive"
        className="min-h-12 w-full"
        onClick={() => {
          setConfirmOpen(true);
        }}
      >
        {copy.action}
      </Button>
      <ConfirmDialog
        open={confirmOpen}
        onClose={() => {
          setConfirmOpen(false);
        }}
        onConfirm={() => {
          cancel.mutate(undefined, {
            onSettled: () => {
              setConfirmOpen(false);
            },
          });
        }}
        title={copy.confirmTitle}
        description={description ?? ''}
        confirmLabel={t.events.cancelConfirmYes}
        variant="destructive"
        isLoading={cancel.isPending}
      />
    </div>
  );
}
