import { Button } from '@/shared/components/ui/button';
import { Badge } from '@/shared/components/ui/badge';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatVenueDayTime } from '@/shared/lib/formatVenueDayTime';
import { formatPrice } from '@/shared/lib/utils';
import type { Sale } from '@/shared/types/api.types';

const t = ES_AR;

interface SaleRowProps {
  sale: Sale;
  onVoid: () => void;
}

export function SaleRow({ sale, onVoid }: SaleRowProps) {
  const isVoided = !!sale.voided_at;

  return (
    <div
      data-testid="sale-row"
      className="border-border-subtle flex items-start justify-between gap-3 border-b py-3 last:border-b-0"
    >
      <div className="min-w-0 space-y-0.5">
        <div className="flex flex-wrap items-center gap-1.5">
          <span className="text-text-tertiary text-xs">{formatVenueDayTime(sale.created_at)}</span>
          <span className="text-text-tertiary text-xs">· {t.bookings.paymentMethods[sale.method]}</span>
          {isVoided && <Badge variant="outline">{t.cash.saleVoidedBadge}</Badge>}
        </div>
        <ul className="text-text-secondary text-sm">
          {sale.items.map((item, index) => (
            <li key={`${item.product_name}-${String(index)}`}>{`${String(item.quantity)}× ${item.product_name}`}</li>
          ))}
        </ul>
      </div>

      <div className="flex shrink-0 flex-col items-end gap-1.5">
        <span className="text-text-primary text-sm font-semibold whitespace-nowrap">{formatPrice(sale.total)}</span>
        {!isVoided && (
          <Button variant="ghost" size="xs" onClick={onVoid}>
            {t.cash.saleVoidAction}
          </Button>
        )}
      </div>
    </div>
  );
}
