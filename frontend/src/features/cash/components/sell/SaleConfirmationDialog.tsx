import { AlertTriangle } from 'lucide-react';
import { Dialog, DialogContent, DialogHeader, DialogTitle } from '@/shared/components/common/AppDialog';
import { SectionFooter } from '@/shared/components/common/SectionFooter';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import type { Sale, SaleStockWarning } from '@/shared/types/api.types';

const t = ES_AR;

export interface SaleResult {
  sale: Sale;
  stockWarnings: SaleStockWarning[];
}

interface SaleConfirmationDialogProps {
  result: SaleResult | null;
  onClose: () => void;
}

/**
 * Shown after a successful charge — the SERVER total and method, never the
 * client's own computed total (`odd/tasks/pos-cashbox.md` T5b). Rendered one
 * level above the grid/cart it was opened from (see `CashSellPage`), the same
 * "close result unmount" lesson `ClosedSessionResult`/`CashCloseDialogSlot`
 * already apply: the products invalidation this sale triggers refetches the
 * grid underneath, and this dialog's own state must not live inside a view
 * that refetch could unmount.
 */
export function SaleConfirmationDialog({ result, onClose }: SaleConfirmationDialogProps) {
  const open = !!result;

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

            <div className="space-y-2">
              <div className="flex items-center justify-between">
                <span className="text-text-secondary text-sm">{t.cash.saleConfirmationTotal}</span>
                <span className="score-text text-text-primary text-lg font-bold">{formatPrice(result.sale.total)}</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-text-secondary text-sm">{t.cash.saleConfirmationMethod}</span>
                <span className="text-text-primary text-sm font-medium">
                  {t.bookings.paymentMethods[result.sale.method]}
                </span>
              </div>
            </div>

            {result.stockWarnings.length > 0 && (
              <div className="border-warning-border bg-warning-bg text-warning-text space-y-1.5 rounded-xl border px-3.5 py-2.5 text-sm">
                <p className="flex items-center gap-1.5 font-medium">
                  <AlertTriangle className="size-4 shrink-0" aria-hidden="true" />
                  {t.cash.saleConfirmationStockWarningTitle}
                </p>
                <ul className="space-y-0.5 pl-1">
                  {result.stockWarnings.map((warning) => (
                    <li key={warning.product_id}>
                      {warning.product_name}: {String(warning.stock_on_hand)}
                    </li>
                  ))}
                </ul>
              </div>
            )}

            <SectionFooter onSubmit={onClose} submitLabel={t.cash.saleConfirmationDone} />
          </>
        )}
      </DialogContent>
    </Dialog>
  );
}
