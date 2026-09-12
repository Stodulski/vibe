import { Banknote } from 'lucide-react';
import { formatPrice } from '@/shared/lib/utils';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function BookingSummary({ isDepositPaid, remaining }: { isDepositPaid: boolean; remaining: number }) {
  return (
    <div className="border-border-subtle bg-bg-subtle flex items-center gap-3 rounded-xl border px-4 py-3">
      <div className="bg-success-bg flex size-9 items-center justify-center rounded-lg">
        <Banknote className="text-success-text size-4" />
      </div>
      <div>
        <p className="text-text-tertiary text-xs">
          {isDepositPaid ? t.bookings.remainingBalance : t.bookings.bookingTotal}
        </p>
        <p className="score-text text-text-primary text-lg font-bold">{formatPrice(remaining)}</p>
      </div>
    </div>
  );
}
