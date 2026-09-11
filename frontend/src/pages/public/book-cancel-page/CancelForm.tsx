import { AlertTriangle } from 'lucide-react';
import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import type { CancelInfoResponse } from '@/shared/types/api.types';
import { BookingInfoCard, type CancelMoneyBand } from './BookingInfoCard';
import { RefundExpiredNotice } from './RefundExpiredNotice';

const t = ES_AR;

/**
 * The subtitle under the title, driven by the amounts `cancel-info` now
 * answers with. Checked in this order:
 * 1. Refundable with a known amount — the automatic-refund sentence.
 *    `refund_amount` already includes the service fee, so this never implies
 *    the fee is excluded.
 * 2. Refundable but by hand (`refund_method: "manual"`) — a booking paid in
 *    cash does not refund itself, so this must never read like #1.
 * 3. Not refundable any more, but something was paid — say what was paid and
 *    that it will not come back; `RefundExpiredNotice` below still carries
 *    the cancellation-window explanation.
 * 4. Nothing to say about money at all.
 */
function getMoneyBand(cancelInfo: CancelInfoResponse): CancelMoneyBand | null {
  const { can_refund, refund_method, refund_amount, paid_amount } = cancelInfo;
  if (can_refund && refund_amount && refund_amount > 0) {
    return { label: t.publicBooking.refundRowLabel, amount: formatPrice(refund_amount) };
  }
  if (can_refund && refund_method === 'manual') {
    return { note: t.publicBooking.refundManualDescription };
  }
  if (!can_refund && paid_amount && paid_amount > 0) {
    return {
      label: t.publicBooking.paidLabel,
      amount: formatPrice(paid_amount),
      note: t.publicBooking.paidAmountNoRefundSuffix,
    };
  }
  return null;
}

function getConfirmDescription(cancelInfo: CancelInfoResponse, canRefund: boolean): string {
  const isManualRefund = cancelInfo.refund_method === 'manual';
  return canRefund
    ? isManualRefund
      ? t.publicBooking.confirmCancelRefundManualDetail
      : t.publicBooking.confirmCancelRefundDetail
    : `${t.publicBooking.cancelNoRefundDetail} ${t.publicBooking.noMoneyBack}.`;
}

interface CancelFormProps {
  cancelInfo: CancelInfoResponse;
  canRefund: boolean;
  confirmOpen: boolean;
  isCancelling: boolean;
  onOpenConfirm: () => void;
  onCloseConfirm: () => void;
  onConfirmCancel: () => void;
  onBack: () => void;
}

export function CancelForm({
  cancelInfo,
  canRefund,
  confirmOpen,
  isCancelling,
  onOpenConfirm,
  onCloseConfirm,
  onConfirmCancel,
  onBack,
}: CancelFormProps) {
  const money = getMoneyBand(cancelInfo);
  const confirmDescription = getConfirmDescription(cancelInfo, canRefund);

  return (
    <div className="mx-auto flex w-full max-w-md flex-col gap-8 px-4 py-10">
      <div className="flex flex-col items-center gap-3 text-center">
        <div className="flex size-16 items-center justify-center rounded-full bg-warning-bg ring-4 ring-warning-border/20">
          <AlertTriangle className="size-8 text-warning-text" />
        </div>
        <h2 className="text-xl font-bold text-text-primary sm:text-2xl">{t.publicBooking.cancelBookingQuestion}</h2>
      </div>

      <BookingInfoCard cancelInfo={cancelInfo} money={money} />

      {!canRefund && cancelInfo.refund_method !== 'none' && (
        <RefundExpiredNotice cancellationHours={cancelInfo.cancellation_hours} />
      )}

      <div className="space-y-3">
        <Button
          onClick={onOpenConfirm}
          className="min-h-12 w-full rounded-xl bg-destructive text-white hover:bg-destructive/90"
        >
          {canRefund ? t.publicBooking.cancelBookingConfirm : t.publicBooking.cancelNoRefund}
        </Button>
        <Button variant="ghost" className="min-h-12 w-full text-text-secondary hover:bg-transparent" onClick={onBack}>
          {t.common.back}
        </Button>
      </div>

      <ConfirmDialog
        open={confirmOpen}
        onClose={onCloseConfirm}
        onConfirm={onConfirmCancel}
        title={canRefund ? t.publicBooking.confirmCancelTitle : t.publicBooking.noRefundTitle}
        description={confirmDescription}
        confirmLabel={canRefund ? t.publicBooking.confirmCancelYes : t.publicBooking.confirmCancelNoRefund}
        variant="destructive"
        isLoading={isCancelling}
      />
    </div>
  );
}
