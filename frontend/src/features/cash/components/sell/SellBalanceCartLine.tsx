import { AlertTriangle, X } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { bookingBalanceLabel, type BalanceLine } from '../../lib/balanceCart';

const t = ES_AR;

interface SellBalanceCartLineProps {
  line: BalanceLine;
  /** A refused checkout's message for this balance, if any (e.g. the amount no longer matches). */
  error?: string | undefined;
  onRemove: (bookingId: string) => void;
}

/**
 * One booking balance on the cart: its label, the amount to collect, and remove.
 * No quantity control: a balance is collected whole or not at all.
 */
export function SellBalanceCartLine({ line, error, onRemove }: SellBalanceCartLineProps) {
  const label = bookingBalanceLabel(line.courtName, line.startTime, line.clientName);

  return (
    <div data-testid="sell-balance-line" className="border-border-subtle space-y-1 border-b py-5 last:border-b-0">
      <div className="flex items-start justify-between gap-3">
        <p className="text-text-primary text-sm font-semibold">{t.cash.balanceLineTitle}</p>
        <span className="text-text-primary text-sm font-semibold whitespace-nowrap tabular-nums">
          {formatPrice(line.amount)}
        </span>
      </div>
      <div className="flex items-center justify-between gap-3">
        <p className="text-text-secondary min-w-0 text-xs break-words">
          {`${line.courtName} · ${line.startTime} · ${line.clientName}`}
        </p>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          className="text-text-tertiary hover:text-text-primary size-11 lg:size-8"
          aria-label={`${t.cash.cartRemove}: ${label}`}
          onClick={() => {
            onRemove(line.bookingId);
          }}
        >
          <X className="size-4" aria-hidden="true" />
        </Button>
      </div>

      {error && (
        <p role="alert" className="text-error-text mt-1.5 flex items-center gap-1 text-xs">
          <AlertTriangle className="size-3.5 shrink-0" aria-hidden="true" />
          {error}
        </p>
      )}
    </div>
  );
}
