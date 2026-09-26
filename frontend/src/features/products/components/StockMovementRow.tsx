import { cn } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatVenueDayTime } from '@/shared/lib/formatVenueDayTime';
import type { StockMovement } from '@/shared/types/api.types';

const t = ES_AR;

function signedQuantity(quantity: number): string {
  return quantity > 0 ? `+${String(quantity)}` : String(quantity);
}

/** One stock movement — date, kind, signed quantity, reason (adjustments only) and note. */
export function StockMovementRow({ movement }: { movement: StockMovement }) {
  return (
    <div className="border-border-subtle flex items-center justify-between gap-3 border-b py-3 last:border-b-0 sm:px-3.5">
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <span className="text-text-primary text-sm font-medium">{t.products.movementKinds[movement.kind]}</span>
          {movement.reason && <span className="text-text-tertiary text-xs">{t.products.reasons[movement.reason]}</span>}
        </div>
        <p className="text-text-tertiary mt-0.5 text-xs">{formatVenueDayTime(movement.created_at)}</p>
        {movement.note && <p className="text-text-secondary mt-1 text-xs">{movement.note}</p>}
      </div>
      <span
        className={cn(
          'score-text shrink-0 text-sm font-semibold',
          movement.quantity > 0 ? 'text-success-text' : movement.quantity < 0 ? 'text-error-text' : 'text-text-primary',
        )}
      >
        {signedQuantity(movement.quantity)}
      </span>
    </div>
  );
}
