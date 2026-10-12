import { AlertTriangle, Check } from 'lucide-react';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { Panel } from '@/shared/components/common/Panel';
import { SkeletonTable } from '@/shared/components/common/Skeletons';
import { StaleDataNotice } from '@/shared/components/common/StaleDataNotice';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import type { BalanceDue } from '@/shared/types/api.types';

const t = ES_AR;

interface BookingBalanceTileProps {
  booking: BalanceDue;
  inCart: boolean;
  onAdd: (booking: BalanceDue) => void;
}

/**
 * One booking that still owes money. The whole tile is the tap target (44px or
 * taller). Once it is in the cart the tile is disabled and says so, so the same
 * booking cannot be added twice.
 */
function BookingBalanceTile({ booking, inCart, onAdd }: BookingBalanceTileProps) {
  return (
    <button
      type="button"
      disabled={inCart}
      onClick={() => {
        onAdd(booking);
      }}
      className="focus-self border-border-subtle bg-bg-subtle hover:border-border-default hover:bg-bg-highlight focus-visible:border-primary-400 active:border-primary-400 disabled:hover:border-border-subtle disabled:hover:bg-bg-subtle flex min-h-11 w-full touch-manipulation flex-col items-start gap-1.5 rounded-2xl border p-3.5 text-left transition-colors duration-150 active:scale-[0.98] disabled:cursor-default disabled:opacity-70"
    >
      <span className="flex w-full items-baseline justify-between gap-2">
        <span className="text-text-primary text-sm font-semibold">{booking.court_name}</span>
        <span className="text-text-tertiary text-xs whitespace-nowrap">{`${booking.start_time} a ${booking.end_time}`}</span>
      </span>
      <span className="text-text-secondary w-full truncate text-sm">{booking.client_name}</span>
      <span className="mt-auto flex w-full items-center justify-between gap-2 pt-1">
        {inCart ? (
          <span className="text-text-tertiary flex items-center gap-1 text-xs">
            <Check className="size-3.5" aria-hidden="true" />
            {t.cash.bookingInCart}
          </span>
        ) : (
          <span />
        )}
        <span className="score-text text-text-primary text-sm font-bold whitespace-nowrap">
          {`${t.cash.bookingRemaining} ${formatPrice(booking.remaining)}`}
        </span>
      </span>
    </button>
  );
}

interface SellBookingBalancesProps {
  bookings: BalanceDue[];
  /** Ids of the bookings already on the cart, to show them as added. */
  inCartIds: ReadonlySet<string>;
  isLoading: boolean;
  isError: boolean;
  onAdd: (booking: BalanceDue) => void;
  onRetry: () => void;
}

/**
 * "Turnos de hoy": today's bookings that still owe money, above the product
 * grid. Tapping one adds its remaining balance to the cart.
 */
export function SellBookingBalances({
  bookings,
  inCartIds,
  isLoading,
  isError,
  onAdd,
  onRetry,
}: SellBookingBalancesProps) {
  return (
    <section className="min-w-0" data-testid="sell-bookings-today">
      <h2 className="text-text-tertiary mb-2 text-sm font-semibold tracking-wider uppercase">{t.cash.bookingsTitle}</h2>

      {isLoading ? (
        <SkeletonTable rows={2} mobile="flat" />
      ) : isError && bookings.length === 0 ? (
        <EmptyState
          icon={AlertTriangle}
          title={t.cash.bookingsLoadError}
          description={t.common.loadErrorDescription}
          actionLabel={t.layout.retry}
          onAction={onRetry}
        />
      ) : bookings.length === 0 ? (
        <Panel size="sm">
          <p className="text-text-tertiary py-6 text-center text-sm">{t.cash.bookingsEmpty}</p>
        </Panel>
      ) : (
        <div className="space-y-3">
          {isError && <StaleDataNotice onRetry={onRetry} />}
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
            {bookings.map((booking) => (
              <BookingBalanceTile
                key={booking.booking_id}
                booking={booking}
                inCart={inCartIds.has(booking.booking_id)}
                onAdd={onAdd}
              />
            ))}
          </div>
        </div>
      )}
    </section>
  );
}
