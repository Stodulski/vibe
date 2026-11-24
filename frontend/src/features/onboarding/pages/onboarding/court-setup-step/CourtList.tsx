import { CheckCircle2, CircleDashed, Trash2 } from 'lucide-react';
import { courtDetailsLine } from '@/features/courts';
import { ES_AR } from '@/shared/i18n/es_AR';
import { cn } from '@/shared/lib/utils';
import type { CourtWithPrices } from '@/shared/types/api.types';

const t = ES_AR;

type CourtLike = Pick<CourtWithPrices, 'id' | 'name' | 'sport' | 'court_type' | 'surface' | 'has_lighting' | 'prices'>;

/**
 * Same notion as the courts page's PriceSummary: no price, not bookable, so
 * not drawn as done.
 */
function isUnpriced(court: CourtLike) {
  return court.prices.length === 0;
}

/** Wraps so `UnpricedRow` can take a row of its own under the court. */
function cardClass(court: CourtLike) {
  return cn(
    'flex flex-wrap items-center justify-between gap-3 rounded-xl border px-4 py-3',
    isUnpriced(court) ? 'border-warning-border bg-warning-bg' : 'border-primary-500/20 bg-primary-500/5',
  );
}

export function CourtList<T extends CourtLike>({
  courts,
  onRemove,
  removePending,
  onSetPrices,
}: {
  courts: T[];
  onRemove: (courtId: string) => void;
  removePending: boolean;
  /** Reopens the price dialog, which the owner may have closed after creating. */
  onSetPrices: (court: T) => void;
}) {
  return (
    <div className="mb-5 space-y-2 sm:mb-6">
      {courts.map((court, i) => (
        <div key={court.id} className={cardClass(court)} style={{ animationDelay: `${String(i * 50)}ms` }}>
          <div className="flex min-w-0 flex-1 items-center gap-3">
            <div className="bg-primary-500/15 flex size-8 shrink-0 items-center justify-center rounded-lg">
              {isUnpriced(court) ? (
                <CircleDashed className="text-warning-icon size-4" />
              ) : (
                <CheckCircle2 className="text-primary-400 size-4" />
              )}
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-text-primary truncate text-sm font-medium">{court.name}</p>
              <p className="text-text-tertiary text-xs">
                {t.courts.sportTypes[court.sport]} · {t.courts.courtTypes[court.court_type]}
              </p>
              {courtDetailsLine(court) && (
                <p className="text-text-tertiary truncate text-xs">{courtDetailsLine(court)}</p>
              )}
            </div>
          </div>
          <button
            type="button"
            onClick={() => {
              onRemove(court.id);
            }}
            disabled={removePending}
            aria-label={`${t.common.delete}: ${court.name}`}
            className="text-text-tertiary hover:bg-error-bg hover:text-error-text flex size-8 shrink-0 items-center justify-center rounded-lg transition-colors disabled:opacity-50"
          >
            <Trash2 className="size-3.5" />
          </button>
          {isUnpriced(court) && (
            <UnpricedRow
              courtName={court.name}
              onSetPrices={() => {
                onSetPrices(court);
              }}
            />
          )}
        </div>
      ))}
    </div>
  );
}

/**
 * Its own row under the court (`basis-full` wraps it in the card's flex row),
 * aligned with the name from sm (at 320px the indent would truncate the notice),
 * because beside the name there was no width for it at all.
 */
function UnpricedRow({ courtName, onSetPrices }: { courtName: string; onSetPrices: () => void }) {
  return (
    <div className="flex basis-full items-center justify-between gap-3 sm:pl-11">
      <p className="text-warning-text min-w-0 truncate text-xs">{t.courts.noPrices}</p>
      <button
        type="button"
        onClick={onSetPrices}
        aria-label={`${t.courts.setPrices}: ${courtName}`}
        className="text-primary-400 hover:text-primary-300 shrink-0 text-xs font-medium whitespace-nowrap underline-offset-2 hover:underline"
      >
        {t.courts.setPrices}
      </button>
    </div>
  );
}
