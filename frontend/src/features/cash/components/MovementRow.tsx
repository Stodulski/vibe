import { Button } from '@/shared/components/ui/button';
import { Badge } from '@/shared/components/ui/badge';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn, formatInstantTime, formatPrice } from '@/shared/lib/utils';
import type { LedgerRow } from '../lib/movementLedger';
import type { CashMovement } from '@/shared/types/api.types';

const t = ES_AR;

interface MovementRowProps {
  row: LedgerRow;
  /** Omitted on a read-only view (a past, closed session's detail page). */
  onVoid?: ((movement: CashMovement) => void) | undefined;
}

/**
 * One fact per line, stacked: what it was, how it was paid, when, and why
 * it was cancelled, so the details never run together on a narrow screen.
 */
function MovementDetails({ row }: { row: LedgerRow }) {
  const { movement, voidedBy } = row;
  const isVoid = !!movement.voids_movement_id;
  const note = voidedBy ? voidedBy.note : movement.note;

  return (
    <div className="min-w-0 space-y-1">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-text-primary text-sm font-semibold">
          {isVoid ? `${t.cash.voidOfPrefix} ${t.cash.voidOfUnknownMovement}` : t.cash.categories[movement.category]}
        </span>
        {voidedBy && <Badge variant="outline">{t.cash.voidedBadge}</Badge>}
      </div>
      <p className="text-text-secondary text-xs">
        {t.cash.kinds[movement.kind]} · {t.bookings.paymentMethods[movement.method]}
      </p>
      <p className="text-text-secondary text-xs">
        {formatInstantTime(movement.created_at)}
        {voidedBy && ` · ${t.cash.voidedBadge} ${formatInstantTime(voidedBy.created_at)}`}
      </p>
      {note && <p className="text-text-secondary text-xs italic">{note}</p>}
    </div>
  );
}

export function MovementRow({ row, onVoid }: MovementRowProps) {
  const { movement, voidedBy } = row;
  const isVoid = !!movement.voids_movement_id;
  const isIncome = movement.kind === 'income';
  // A 'sale' income is system-written and can only ever be voided by voiding
  // the sale itself (internal/sales.Store.Void restores stock AND voids the
  // income in one transaction) — a manual void here is refused with 409
  // (cashboxstore.ErrCannotVoidSaleManually, backend/internal/cashbox/service.go's
  // VoidMovement). 'restock' has no such restriction and voids like any other
  // manual movement.
  const canVoid = !!onVoid && !isVoid && !voidedBy && movement.category !== 'sale';

  return (
    <div className="border-border-subtle flex items-start justify-between gap-4 border-b py-4 last:border-b-0">
      <MovementDetails row={row} />

      {/* The amount leads; Anular sits under it, small and muted, because
          voiding is the rare, irreversible action on the row. */}
      <div className="flex shrink-0 flex-col items-end gap-1">
        <span
          className={cn(
            'text-sm font-semibold whitespace-nowrap tabular-nums',
            voidedBy ? 'text-text-tertiary line-through' : isIncome ? 'text-success-text' : 'text-error-text',
          )}
        >
          {isIncome ? '+' : '-'}
          {formatPrice(movement.amount)}
        </span>
        {canVoid && (
          <Button
            variant="link"
            size="xs"
            className="text-text-tertiary hover:text-text-secondary h-auto px-0 py-1 font-normal"
            onClick={() => {
              onVoid(movement);
            }}
          >
            {t.cash.voidAction}
          </Button>
        )}
      </div>
    </div>
  );
}
