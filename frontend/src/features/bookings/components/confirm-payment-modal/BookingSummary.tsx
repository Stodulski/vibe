import { Banknote } from 'lucide-react';
import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function BookingSummary({ isDepositPaid, remaining }: { isDepositPaid: boolean; remaining: number }) {
  return (
    <div className="flex items-center gap-3 rounded-xl border border-border-subtle bg-bg-subtle px-4 py-3">
      <div className="flex size-9 items-center justify-center rounded-lg bg-success-bg">
        <Banknote className="size-4 text-success-text" />
      </div>
      <div>
        <p className="text-xs text-text-tertiary">
          {isDepositPaid ? t.bookings.remainingBalance : t.bookings.bookingTotal}
        </p>
        <p className="score-text text-lg font-bold text-text-primary">{formatPrice(remaining)}</p>
      </div>
    </div>
  );
}
