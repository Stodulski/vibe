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
    <div data-testid="sell-balance-line" className="border-border-subtle border-b py-3 last:border-b-0">
      <div className="flex items-start justify-between gap-3">
        <p className="text-text-primary min-w-0 flex-1 text-sm font-medium break-words">{label}</p>
        <Button
          type="button"
          variant="ghost"
          size="icon-sm"
          className="size-11 lg:size-8"
          aria-label={`${t.cash.cartRemove}: ${label}`}
          onClick={() => {
            onRemove(line.bookingId);
          }}
        >
          <X className="size-4" aria-hidden="true" />
        </Button>
      </div>

      <p className="score-text text-text-primary mt-2 text-right text-sm font-semibold">{formatPrice(line.amount)}</p>

      {error && (
        <p role="alert" className="text-error-text mt-1.5 flex items-center gap-1 text-xs">
          <AlertTriangle className="size-3.5 shrink-0" aria-hidden="true" />
          {error}
        </p>
      )}
    </div>
  );
}
