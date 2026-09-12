import { Loader2, Lock } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';

const t = ES_AR;

interface SubmitFooterProps {
  isLoading: boolean;
  totalOnline: number;
  type: 'submit' | 'button';
  onClick?: () => void;
}

/**
 * The button used to read `` `${payDepositAmount} · ${formatPrice(totalOnline)}` ``
 * — "Pagar · $1.010" — which wrapped onto two lines on a phone-width button.
 * The owner's fix: a padlock plus "Pagar de manera segura", always one line,
 * with the amount moved beside the button instead of concatenated into its
 * label. `BookingSummaryCard` already states the total above the form, but
 * on a phone that card can be a scroll away by the time the visitor reaches
 * this button, so it is restated here, right next to the action it belongs to.
 */
export function SubmitFooter({ isLoading, totalOnline, type, onClick }: SubmitFooterProps) {
  return (
    <div className="space-y-2 pt-1">
      <div className="flex items-baseline justify-between text-sm">
        <span className="text-text-secondary">{t.publicBooking.totalOnline}</span>
        <span className="text-text-primary font-semibold tabular-nums">{formatPrice(totalOnline)}</span>
      </div>
      <Button
        type={type}
        className="shadow-brand h-14 w-full rounded-xl text-base font-semibold"
        disabled={isLoading}
        aria-busy={isLoading}
        onClick={onClick}
      >
        {isLoading ? (
          <>
            <Loader2 className="size-5 animate-spin" aria-hidden="true" />
            <span className="sr-only">{t.publicBooking.generatingPaymentLink}</span>
          </>
        ) : (
          <>
            <Lock className="size-4 shrink-0" aria-hidden="true" />
            {t.publicBooking.paySecurely}
          </>
        )}
      </Button>
    </div>
  );
}
