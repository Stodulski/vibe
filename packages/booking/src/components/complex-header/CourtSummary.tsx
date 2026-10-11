import { formatPrice } from '@vibe/ui';
import type { CourtWithPrices } from '../../api/types';
import { sportSummaries } from '../../lib/courtPrices';
import { BOOKING_MESSAGES } from '../../messages';

const t = BOOKING_MESSAGES;

/**
 * How many courts of each sport the club has, and the cheapest hour of each.
 *
 * Plain text in the server markup, so the page states its courts and prices
 * before any script runs. A sport with no priced band shows its count alone.
 */
export function CourtSummary({ courts }: { courts: readonly CourtWithPrices[] }) {
  const summaries = sportSummaries(courts);
  if (summaries.length === 0) return null;

  return (
    <ul className="border-border-subtle space-y-2 border-t pt-5 pb-5 text-sm">
      {summaries.map(({ sport, courts: count, minPrice }) => {
        const name = t.courts.sportTypes[sport].toLowerCase();
        const label = count === 1 ? `1 cancha de ${name}` : `${String(count)} canchas de ${name}`;
        return (
          <li key={sport} className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
            <span className="text-text-primary font-medium">{label}</span>
            {minPrice !== null && (
              <span className="text-text-secondary tabular-nums">{`desde ${formatPrice(minPrice)} la hora`}</span>
            )}
          </li>
        );
      })}
    </ul>
  );
}
