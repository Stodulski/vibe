import { useId, useRef, useState, type ReactNode } from 'react';
import { AlertTriangle, Check, Search } from 'lucide-react';
import { IconInput } from '@/shared/components/common/IconInput';
import { useDragScroll } from '@/shared/hooks/useDragScroll';
import { cn } from '@/shared/lib/utils';
import { EmptyState } from '@/shared/components/common/EmptyState';
import { StaleDataNotice } from '@/shared/components/common/StaleDataNotice';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import type { BalanceDue } from '@/shared/types/api.types';

const t = ES_AR;

interface BookingBalanceTileProps {
  booking: BalanceDue;
  inCart: boolean;
  onAdd: (booking: BalanceDue) => void;
  onRemove: (bookingId: string) => void;
}

/**
 * One booking that still owes money. The whole tile is the tap target (44px or
 * taller) and toggles it: a tap adds its balance to the cart, a second tap
 * takes it out again. A balance is collected whole, so it is never added twice.
 */
function BookingBalanceTile({ booking, inCart, onAdd, onRemove }: BookingBalanceTileProps) {
  return (
    <button
      type="button"
      aria-pressed={inCart}
      onFocus={(event) => {
        // A card the row shows only at its edge is not scrolled into view by keyboard
        // focus on its own, so the focused card is brought in whole.
        event.currentTarget.scrollIntoView({ block: 'nearest', inline: 'nearest' });
      }}
      onClick={() => {
        if (inCart) onRemove(booking.booking_id);
        else onAdd(booking);
      }}
      className={`focus-self ${inCart ? 'border-primary-500 hover:border-primary-400' : 'border-border-subtle hover:border-border-default'} bg-bg-subtle hover:bg-bg-highlight focus-visible:border-primary-400 active:border-primary-400 flex h-28 min-h-11 w-full touch-manipulation flex-col items-start gap-1.5 rounded-2xl border p-3.5 text-left transition-colors duration-150 active:scale-[0.98]`}
    >
      <span className="flex w-full items-baseline justify-between gap-2">
        <span className="text-text-primary text-sm font-semibold">{booking.court_name}</span>
        <span className="text-text-tertiary text-xs whitespace-nowrap">{`${booking.start_time} a ${booking.end_time}`}</span>
      </span>
      <span className="text-text-secondary w-full truncate text-sm">{booking.client_name}</span>
      <span className="mt-auto flex w-full items-center justify-between gap-2 pt-1">
        {inCart ? (
          <span className="text-primary-400 flex items-center gap-1 text-xs font-medium">
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
  onRemove: (bookingId: string) => void;
  onRetry: () => void;
}

/**
 * The bookings as one horizontal slider at every width, so a busy day never
 * pushes the product grid down. A card past the edge peeks in, the thin green
 * bar below shows there is more, and a mouse can drag it (`useDragScroll`).
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
  const ref = useRef<HTMLDivElement>(null);
  const { dragging, dragHandlers } = useDragScroll(ref);

  return (
    <div
      ref={ref}
      className={cn(
        'scrollbar-accent overflow-x-auto overscroll-x-contain pb-3 select-none',
        dragging ? 'cursor-grabbing snap-none' : 'snap-x snap-mandatory',
      )}
      {...dragHandlers}
    >
      {/* Tailwind's preflight sets list-style: none, which makes Safari drop the
          list semantics, so the role stays explicit. */}
      {/* eslint-disable-next-line jsx-a11y/no-redundant-roles */}
      <ul role="list" aria-labelledby={labelledBy} aria-busy={busy} className="flex gap-3">
        {children}
      </ul>
    </div>
  );
}

/** One card's slot in the row. */
function BookingsRowItem({ children }: { children: ReactNode }) {
  return <li className="w-60 shrink-0 snap-start lg:w-64">{children}</li>;
}

/** Client or court containing the typed text, ignoring case and accents. */
function matchesBooking(booking: BalanceDue, term: string): boolean {
  const fold = (value: string) =>
    value
      .normalize('NFD')
      .replace(/\p{Diacritic}/gu, '')
      .toLowerCase();
  const needle = fold(term.trim());
  return needle === '' || fold(`${booking.client_name} ${booking.court_name}`).includes(needle);
}

interface BookingsListProps {
  bookings: BalanceDue[];
  inCartIds: ReadonlySet<string>;
  isError: boolean;
  headingId: string;
  onAdd: (booking: BalanceDue) => void;
  onRemove: (bookingId: string) => void;
  onRetry: () => void;
}

/** The search box and the slider of matching bookings. */
function BookingsList({ bookings, inCartIds, isError, headingId, onAdd, onRemove, onRetry }: BookingsListProps) {
  const [search, setSearch] = useState('');
  const visible = bookings.filter((booking) => matchesBooking(booking, search));

  return (
    <div className="space-y-3">
      {isError && <StaleDataNotice onRetry={onRetry} />}
      <IconInput
        icon={Search}
        value={search}
        onChange={(e) => {
          setSearch(e.target.value);
        }}
        placeholder={t.cash.bookingsSearchPlaceholder}
        aria-label={t.cash.bookingsSearchPlaceholder}
      />
      {visible.length === 0 ? (
        <p className="text-text-secondary text-sm">{t.cash.bookingsNoResults}</p>
      ) : (
        <BookingsRow labelledBy={headingId}>
          {visible.map((booking) => (
            <BookingsRowItem key={booking.booking_id}>
              <BookingBalanceTile
                booking={booking}
                inCart={inCartIds.has(booking.booking_id)}
                onAdd={onAdd}
                onRemove={onRemove}
              />
            </BookingsRowItem>
          ))}
        </BookingsRow>
      )}
    </div>
  );
}

/**
 * "Turnos de hoy": today's bookings that still owe money, above the product
 * grid. Tapping one puts its remaining balance on the cart, replacing any
 * other: one booking per sale.
 */
export function SellBookingBalances({
  bookings,
  inCartIds,
  isLoading,
  isError,
  onAdd,
  onRemove,
  onRetry,
}: SellBookingBalancesProps) {
  const headingId = useId();

  return (
    <section className="min-w-0" data-testid="sell-bookings-today" aria-labelledby={headingId}>
      {/* Visually hidden so the cards line up with the cart's top edge; screen
          readers still get the heading the list is labelled by. */}
      <h2 id={headingId} className="sr-only">
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
      ) : isError && bookings.length === 0 ? (
        <EmptyState
          icon={AlertTriangle}
          title={t.cash.bookingsLoadError}
          description={t.common.loadErrorDescription}
          actionLabel={t.layout.retry}
          onAction={onRetry}
        />
      ) : bookings.length === 0 ? (
        <p className="text-text-secondary text-sm">{t.cash.bookingsEmpty}</p>
      ) : (
        <BookingsList
          bookings={bookings}
          inCartIds={inCartIds}
          isError={isError}
          headingId={headingId}
          onAdd={onAdd}
          onRemove={onRemove}
          onRetry={onRetry}
        />
      )}
    </section>
  );
}
