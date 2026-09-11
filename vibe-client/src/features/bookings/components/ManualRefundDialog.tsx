import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';

const t = ES_AR;

interface ManualRefundDialogProps {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  isLoading: boolean;
  amount: number;
}

export function ManualRefundDialog({ open, onClose, onConfirm, isLoading, amount }: ManualRefundDialogProps) {
  return (
    <ConfirmDialog
      open={open}
      onClose={onClose}
      onConfirm={onConfirm}
      title={t.bookings.markManualRefund}
      description={`${t.bookings.manualRefundConfirmBefore} ${formatPrice(amount)} ${t.bookings.manualRefundConfirmAfter}`}
      confirmLabel={t.bookings.markManualRefund}
      variant="default"
      isLoading={isLoading}
    />
  );
}
