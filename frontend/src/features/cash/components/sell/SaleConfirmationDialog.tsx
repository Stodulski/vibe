import { AlertTriangle } from 'lucide-react';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/shared/components/common/AppDialog';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import type { CounterPaymentMethod } from '@/shared/lib/paymentMethods';
import type { Sale, SaleBookingPayment, SaleItem, SaleStockWarning } from '@/shared/types/api.types';
import { bookingBalanceLabel } from '../../lib/balanceCart';

const t = ES_AR;

export interface SaleResult {
  /** The method the checkout was charged with: a booking-only checkout has no sale to read it from. */
  method: CounterPaymentMethod;
  /** Null for a checkout that only collected booking balances. */
  sale: Sale | null;
  bookingPayments: SaleBookingPayment[];
  stockWarnings: SaleStockWarning[];
}

interface SaleLinesProps {
  items: SaleItem[];
  bookingPayments: SaleBookingPayment[];
}

/** What the checkout collected: its product lines, then its booking balances. */
function SaleLines({ items, bookingPayments }: SaleLinesProps) {
  return (
    <>
      {items.length > 0 && (
        <ul className="text-text-secondary space-y-1 text-sm">
          {items.map((item, index) => (
            <li key={`${item.product_name}-${String(index)}`}>{`${String(item.quantity)}× ${item.product_name}`}</li>
          ))}
        </ul>
      )}

      {bookingPayments.length > 0 && (
        <ul className="text-text-secondary space-y-1 text-sm" aria-label={t.cash.saleBookingsTitle}>
          {bookingPayments.map((payment) => (
            <li key={payment.payment_id} className="flex items-start justify-between gap-3">
              <span className="min-w-0">
                {bookingBalanceLabel(payment.court_name, payment.start_time, payment.client_name)}
              </span>
              <span className="text-text-primary shrink-0 font-medium">{formatPrice(payment.amount)}</span>
            </li>
          ))}
        </ul>
      )}
    </>
  );
}

/** The server-side total of everything collected, and the method it was charged with. */
function SaleTotals({ total, method }: { total: number; method: CounterPaymentMethod }) {
  return (
    <div className="space-y-2">
      <div className="flex items-center justify-between">
        <span className="text-text-secondary text-sm">{t.cash.saleConfirmationTotal}</span>
        <span className="score-text text-text-primary text-lg font-bold">{formatPrice(total)}</span>
      </div>
      <div className="flex items-center justify-between">
        <span className="text-text-secondary text-sm">{t.cash.saleConfirmationMethod}</span>
        <span className="text-text-primary text-sm font-medium">{t.bookings.paymentMethods[method]}</span>
      </div>
    </div>
  );
}

function SaleStockWarnings({ warnings }: { warnings: SaleStockWarning[] }) {
  return (
    <div className="border-warning-border bg-warning-bg text-warning-text space-y-1.5 rounded-xl border px-3.5 py-2.5 text-sm">
      <p className="flex items-center gap-1.5 font-medium">
        <AlertTriangle className="size-4 shrink-0" aria-hidden="true" />
        {t.cash.saleConfirmationStockWarningTitle}
      </p>
      <ul className="space-y-0.5 pl-1">
        {warnings.map((warning) => (
          <li key={warning.product_id}>
            {warning.product_name}: {String(warning.stock_on_hand)}
          </li>
        ))}
      </ul>
    </div>
  );
}

interface SaleConfirmationDialogProps {
  result: SaleResult | null;
  onClose: () => void;
}

/**
 * Shown after a successful charge — the SERVER totals and method, never the
 * client's own computed total (`odd/tasks/pos-cashbox.md` T5b). It lists both
 * kinds of line the checkout collected: the products and the booking balances.
 * Rendered one level above the grid/cart it was opened from (see `CashSellPage`),
 * the same "close result unmount" lesson `ClosedSessionResult`/`CashCloseDialogSlot`
 * already apply: the products invalidation this sale triggers refetches the
 * grid underneath, and this dialog's own state must not live inside a view
 * that refetch could unmount.
 */
export function SaleConfirmationDialog({ result, onClose }: SaleConfirmationDialogProps) {
  const open = !!result;
  const productsTotal = result?.sale?.total ?? 0;
  const balancesTotal = result?.bookingPayments.reduce((sum, payment) => sum + payment.amount, 0) ?? 0;

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) onClose();
      }}
    >
      <DialogContent className="sm:max-w-md">
        {result && (
          <>
            <DialogHeader>
              <DialogTitle>{t.cash.saleConfirmationTitle}</DialogTitle>
            </DialogHeader>

            <SaleTotals total={productsTotal + balancesTotal} method={result.method} />

            <SaleLines items={result.sale?.items ?? []} bookingPayments={result.bookingPayments} />

            {result.stockWarnings.length > 0 && <SaleStockWarnings warnings={result.stockWarnings} />}

            <SectionFooter onSubmit={onClose} submitLabel={t.cash.saleConfirmationDone} />
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
