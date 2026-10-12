import { useId, type ReactNode } from 'react';
import { AlertTriangle, Check } from 'lucide-react';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { Panel } from '@/shared/components/common/Panel';
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
      onFocus={(event) => {
        // A card the row shows only at its edge is not scrolled into view by keyboard
        // focus on its own, so the focused card is brought in whole.
        event.currentTarget.scrollIntoView({ block: 'nearest', inline: 'nearest' });
      }}
      onClick={() => {
        onAdd(booking);
      }}
      className="focus-self border-border-subtle bg-bg-subtle hover:border-border-default hover:bg-bg-highlight focus-visible:border-primary-400 active:border-primary-400 disabled:hover:border-border-subtle disabled:hover:bg-bg-subtle flex h-28 min-h-11 w-full touch-manipulation flex-col items-start gap-1.5 rounded-2xl border p-3.5 text-left transition-colors duration-150 active:scale-[0.98] disabled:cursor-default disabled:opacity-70 lg:h-auto"
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
 * Below `lg` the bookings are one horizontal row, so the product grid starts near
 * the top of the page. A card past the edge peeks in to show there is more to
 * swipe to. From `lg` up it is the grid the page has always shown.
 */
function BookingsRow({
  labelledBy,
  busy = false,
  children,
}: {
  labelledBy: string;
  busy?: boolean;
  children: ReactNode;
}) {
  return (
    // Tailwind's preflight sets list-style: none, which makes Safari drop the list
    // semantics, so the role stays explicit.
    // eslint-disable-next-line jsx-a11y/no-redundant-roles
    <ul
      role="list"
      aria-labelledby={labelledBy}
      aria-busy={busy}
      className="flex snap-x snap-mandatory gap-3 overflow-x-auto overscroll-x-contain lg:grid lg:snap-none lg:grid-cols-2 lg:overflow-visible xl:grid-cols-3"
    >
      {children}
    </ul>
  );
}

/** One card's slot in the row: card width below `lg`, a grid cell from `lg` up. */
function BookingsRowItem({ children }: { children: ReactNode }) {
  return <li className="w-60 shrink-0 snap-start lg:w-auto lg:shrink">{children}</li>;
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
  const headingId = useId();

  return (
    <section className="min-w-0" data-testid="sell-bookings-today" aria-labelledby={headingId}>
      <h2 id={headingId} className="text-text-tertiary mb-2 text-sm font-semibold tracking-wider uppercase">
        {t.cash.bookingsTitle}
      </h2>

      {isLoading ? (
        <BookingsRow labelledBy={headingId} busy>
          {[0, 1].map((slot) => (
            <BookingsRowItem key={slot}>
              <div
                aria-hidden="true"
                className="border-border-subtle bg-bg-subtle h-28 animate-pulse rounded-2xl border"
              />
            </BookingsRowItem>
          ))}
        </BookingsRow>
      ) : (
        // The body keeps the card row's height in every state below `lg`, so the
        // product grid below does not move when the bookings arrive.
        <div className="min-h-28 lg:min-h-0">
          {isError && bookings.length === 0 ? (
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
              <BookingsRow labelledBy={headingId}>
                {bookings.map((booking) => (
                  <BookingsRowItem key={booking.booking_id}>
                    <BookingBalanceTile booking={booking} inCart={inCartIds.has(booking.booking_id)} onAdd={onAdd} />
                  </BookingsRowItem>
                ))}
              </BookingsRow>
            </div>
          )}
        </div>
      )}
    </section>
  );
}
