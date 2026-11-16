import { useState, type ReactNode } from 'react';
import { Sheet, SheetContent } from '@/shared/components/ui/sheet';
import { useOutsideClickGrace } from '@/shared/hooks/useOutsideClickGrace';
import { useExitingValue } from '@/shared/hooks/useExitingValue';
import { useBooking } from '../hooks/useBooking';
import { useBookingSeries } from '../hooks/useBookingSeries';
import { BookingSeriesLabel } from './booking-detail/BookingSeriesLabel';
import { EndSeriesDialog } from './booking-detail/EndSeriesDialog';
import { todayInArgentina } from '../lib/today';
import { BookingDetailSheetHeader } from './booking-detail/BookingDetailSheetHeader';
import { BookingInfoSection } from './booking-detail/BookingInfoSection';
import { ClientInfoRow } from './booking-detail/ClientInfoRow';
import { NotesSection } from './booking-detail/NotesSection';
import { BookingDetailFooter } from './booking-detail/BookingDetailFooter';
import { manualRefundOwed } from '../lib/manualRefundOwed';
import { mercadoPagoRefundInFlight } from '../lib/mercadoPagoRefundInFlight';
import { bookingDisplayStatus } from '@/shared/types/api.types';
import type { Booking, BookingSeries, Client, Payment } from '@/shared/types/api.types';

function getBookingDetailFlags(b: Booking, payments: Payment[]) {
  const isCancelled = b.status === 'cancelled';
  const isCompleted = b.status === 'completed';
  const isNoShow = b.status === 'no_show';
  // Once the hours are over there is nothing left to cancel: the owner marks
  // whether the client came instead.
  const isFinished = bookingDisplayStatus(b) === 'finished';
  const canCancel = !isCancelled && !isCompleted && !isNoShow && !isFinished;
  const canMarkNoShow = b.status === 'confirmed' || b.status === 'completed';
  const canMarkAttended = isFinished;
  // The server refuses a payment on a no-show (the deposit is kept) and a
  // manual refund while Mercado Pago is still sending its part back, so
  // neither button is offered then.
  const canConfirmPayment =
    b.collection_status !== 'fully_paid' && b.refund_status === 'none' && !isCancelled && !isNoShow;
  const canMarkManualRefund = isCancelled && b.refund_status === 'partial' && !mercadoPagoRefundInFlight(payments);
  const manualRefundAmount = canMarkManualRefund ? manualRefundOwed(payments) : 0;

  return { canCancel, canMarkNoShow, canMarkAttended, canConfirmPayment, canMarkManualRefund, manualRefundAmount };
}

/** The detail query plus the "keep showing the last thing while it exits"
 *  and "fall back to the list's own booking" adjustments layered on top of it,
 *  pulled out so `BookingDetail` itself reads as the sheet, not the fetch. */
function useBookingDetailState(complexId: string, open: boolean, booking: Booking | null) {
  const {
    data: detail,
    isPending: isDetailPending,
    isError: isDetailError,
    refetch: refetchDetail,
  } = useBooking(complexId, open && booking ? booking.id : null);

  // Retained across the exit: cancelling clears the selected booking in the
  // same tick it closes the sheet, and the detail query goes disabled with it,
  // so both sources drop to nothing at once and the sheet used to disappear
  // mid-slide instead of animating away.
  const b = useExitingValue(detail?.booking ?? booking);
  const seriesQuery = useBookingSeries(complexId, open ? b?.series_id : null);

  return {
    b,
    series: seriesQuery.data?.series,
    isSeriesPending: seriesQuery.isPending && !!b?.series_id,
    client: detail?.client,
    payments: detail?.payments ?? [],
    isDetailPending,
    isDetailError,
    refetchDetail,
  };
}

interface BookingDetailProps {
  open: boolean;
  onClose: () => void;
  booking: Booking | null;
  complexId: string;
  onCancel: (booking: Booking) => void;
  onConfirmPayment: (booking: Booking) => void;
  onNoShow: (booking: Booking) => void;
  onAttended: (booking: Booking) => void;
  onManualRefund: (booking: Booking, amount: number) => void;
  /** Hides the client row — the drawer underneath is already that client. */
  hideClient?: boolean | undefined;
  /** Opens the client's own drawer from the client row. */
  onOpenClient?: ((client: Client) => void) | undefined;
}

/** What the detail shows and offers for a booking that is one occurrence of a fixed weekly series. */
interface SeriesDetail {
  series: BookingSeries | undefined;
  isPending: boolean;
  /** Present while the series can still be ended from this booking's date. */
  onEnd: (() => void) | undefined;
}

interface BookingDetailBodyProps {
  booking: Booking;
  seriesDetail: SeriesDetail | null;
  clientRow: ReactNode;
  payments: Payment[];
  isDetailPending: boolean;
  isDetailError: boolean;
  refetchDetail: () => unknown;
  canMarkNoShow: boolean;
  canMarkAttended: boolean;
  canCancel: boolean;
  onNoShow: (booking: Booking) => void;
  onAttended: (booking: Booking) => void;
  onCancel: (booking: Booking) => void;
  canConfirmPayment: boolean;
  canMarkManualRefund: boolean;
  manualRefundAmount: number;
  onConfirmPayment: (booking: Booking) => void;
  onManualRefund: (booking: Booking, amount: number) => void;
}

function BookingDetailBody({
  booking,
  seriesDetail,
  clientRow,
  payments,
  isDetailPending,
  isDetailError,
  refetchDetail,
  ...actions
}: BookingDetailBodyProps) {
  const isSeries = seriesDetail !== null;
  const onEndSeries = seriesDetail?.onEnd;
  return (
    <>
      <BookingDetailSheetHeader booking={booking} {...actions} isSeries={isSeries} onEndSeries={onEndSeries} />

      {/* Only the body scrolls: the header and the action footer stay put, and
          the bottom padding keeps the last row off the edge. */}
      <div className="min-h-0 flex-1 space-y-8 overflow-y-auto px-4 pt-4 pb-8 sm:px-6">
        <div className="space-y-4">
          {seriesDetail && <BookingSeriesLabel series={seriesDetail.series} isPending={seriesDetail.isPending} />}
          <BookingInfoSection
            booking={booking}
            payments={payments}
            clientRow={clientRow}
            paymentsPending={isDetailPending}
            paymentsError={isDetailError}
            onRetryPayments={() => {
              void refetchDetail();
            }}
          />
        </div>
        {booking.notes && <NotesSection notes={booking.notes} />}
      </div>

      <BookingDetailFooter booking={booking} {...actions} isSeries={isSeries} onEndSeries={onEndSeries} />
    </>
  );
}

/**
 * `seriesDetail` is `null` for an ordinary booking; for an occurrence of a
 * fixed weekly series it carries the series line and the "end from here"
 * action, with the open state of the dialog that confirms it.
 */
function useSeriesDetail(
  booking: Booking | null,
  series: BookingSeries | undefined,
  isPending: boolean,
): { seriesDetail: SeriesDetail | null; endOpen: boolean; closeEnd: () => void } {
  const [endOpen, setEndOpen] = useState(false);
  if (!booking?.series_id) return { seriesDetail: null, endOpen, closeEnd: () => undefined };
  // The server refuses an end date in the past, and there is nothing to end once the series is.
  const canEnd = booking.date.slice(0, 10) >= todayInArgentina() && series?.status !== 'ended';
  return {
    seriesDetail: {
      series,
      isPending,
      onEnd: canEnd
        ? () => {
            setEndOpen(true);
          }
        : undefined,
    },
    endOpen,
    closeEnd: () => {
      setEndOpen(false);
    },
  };
}

/** Where the series ends when it is cancelled from this booking: its own date. */
function seriesEndTarget(booking: Booking) {
  return { seriesId: booking.series_id, fromDate: booking.date.slice(0, 10) };
}

export function BookingDetail({
  open,
  onClose,
  booking,
  complexId,
  hideClient,
  onOpenClient,
  ...handlers
}: BookingDetailProps) {
  const { b, client, payments, isDetailPending, isDetailError, refetchDetail, series, isSeriesPending } =
    useBookingDetailState(complexId, open, booking);
  const { seriesDetail, endOpen, closeEnd } = useSeriesDetail(b, series, isSeriesPending);
  const onPointerDownOutside = useOutsideClickGrace(open);

  if (!b) return null;

  const flags = getBookingDetailFlags(b, payments);

  const clientRow = hideClient ? undefined : <ClientInfoRow booking={b} client={client} onOpenClient={onOpenClient} />;

  return (
    <>
      <Sheet
        open={open}
        onOpenChange={(v) => {
          if (!v) onClose();
        }}
      >
        <SheetContent
          side="right"
          className="sm:max-w-lg"
          showCloseButton={false}
          onPointerDownOutside={onPointerDownOutside}
        >
          <BookingDetailBody
            booking={b}
            seriesDetail={seriesDetail}
            clientRow={clientRow}
            payments={payments}
            isDetailPending={isDetailPending}
            isDetailError={isDetailError}
            refetchDetail={refetchDetail}
            {...flags}
            {...handlers}
          />
        </SheetContent>
      </Sheet>
      <EndSeriesDialog
        {...seriesEndTarget(b)}
        complexId={complexId}
        open={endOpen}
        onClose={closeEnd}
        onEnded={onClose}
      />
    </>
  );
}
