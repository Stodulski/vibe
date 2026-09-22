import { Button } from '@/shared/components/ui/button';
import { Badge } from '@/shared/components/ui/badge';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn, formatInstantTime, formatPrice } from '@/shared/lib/utils';
import type { CashMovement } from '@/shared/types/api.types';

const t = ES_AR;

interface MovementRowProps {
  movement: CashMovement;
  /** The full ledger of the session `movement` belongs to — void linkage never crosses sessions (see `useVoidCashMovement`: voiding is only ever offered for the currently open session's own movements). */
  movements: CashMovement[];
  /** Omitted on a read-only view (a past, closed session's detail page). */
  onVoid?: ((movement: CashMovement) => void) | undefined;
}

export function MovementRow({ movement, movements, onVoid }: MovementRowProps) {
  const isVoid = !!movement.voids_movement_id;
  const isVoided = movements.some((m) => m.voids_movement_id === movement.id);
  const original = isVoid ? movements.find((m) => m.id === movement.voids_movement_id) : undefined;
  const isIncome = movement.kind === 'income';

  return (
    <div className="border-border-subtle flex items-start justify-between gap-3 border-b py-3 last:border-b-0">
      <div className="min-w-0 space-y-0.5">
        <div className="flex flex-wrap items-center gap-1.5">
          <Badge variant={isIncome ? 'default' : 'secondary'} className="text-micro">
            {t.cash.kinds[movement.kind]}
          </Badge>
          <span className="text-text-primary text-sm font-medium">{t.cash.categories[movement.category]}</span>
          <span className="text-text-tertiary text-xs">· {t.bookings.paymentMethods[movement.method]}</span>
          {isVoided && <Badge variant="outline">{t.cash.voidedBadge}</Badge>}
        </div>
        <p className="text-text-tertiary text-xs">
          {formatInstantTime(movement.created_at)}
          {isVoid &&
            ` · ${t.cash.voidOfPrefix} ${original ? t.cash.categories[original.category] : t.cash.voidOfUnknownMovement}`}
        </p>
        {movement.note && <p className="text-text-tertiary text-xs italic">{movement.note}</p>}
      </div>

      <div className="flex shrink-0 flex-col items-end gap-1.5">
        <span
          className={cn('text-sm font-semibold whitespace-nowrap', isIncome ? 'text-success-text' : 'text-error-text')}
        >
          {isIncome ? '+' : '-'}
          {formatPrice(movement.amount)}
        </span>
        {onVoid && !isVoid && !isVoided && (
          <Button
            variant="ghost"
            size="xs"
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
