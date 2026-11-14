import { formatPrice } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';

const t = BOOKING_MESSAGES;

interface PriceSummaryProps {
  /** What the entry is called: "Lugar", "Inscripción de la pareja". */
  label: string;
  /** Centavos ARS. */
  price: number;
  serviceFee: number;
  total: number;
}

/** The entry, the service fee on top and the total the person pays online. */
export function PriceSummary({ label, price, serviceFee, total }: PriceSummaryProps) {
  return (
    <dl className="border-border-subtle flex flex-col gap-2 border-t pt-3 text-sm">
      <div className="flex items-baseline justify-between">
        <dt className="text-text-secondary">{label}</dt>
        <dd className="text-text-primary tabular-nums">{formatPrice(price)}</dd>
      </div>
      <div className="flex items-baseline justify-between">
        <dt className="text-text-secondary">{t.serviceFee.label}</dt>
        <dd className="text-text-primary tabular-nums">{formatPrice(serviceFee)}</dd>
      </div>
      <div className="flex items-baseline justify-between">
        <dt className="text-text-secondary">{t.publicBooking.totalOnline}</dt>
        <dd className="text-text-primary font-semibold tabular-nums">{formatPrice(total)}</dd>
      </div>
    </dl>
  );
}
