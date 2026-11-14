import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useEndBookingSeries } from '../../hooks/useEndBookingSeries';
import type { Booking } from '@/shared/types/api.types';

const t = ES_AR;

/**
 * Confirms ending a fixed weekly booking from one date on. The server cancels
 * the unpaid occurrences from that date and keeps the paid ones, which is what
 * the description tells the owner before they confirm. `onClose` shuts this
 * dialog, `onEnded` the booking sheet behind it.
 */
export function EndSeriesDialog({
  open,
  onClose,
  onEnded,
  complexId,
  booking,
}: {
  open: boolean;
  onClose: () => void;
  onEnded: () => void;
  complexId: string;
  /** The occurrence the owner is looking at: its date is where the series ends. */
  booking: Booking;
}) {
  const endSeries = useEndBookingSeries(complexId);
  if (!booking.series_id) return null;
  const seriesId = booking.series_id;
  const fromDate = booking.date.slice(0, 10);

  return (
    <ConfirmDialog
      open={open}
      onClose={onClose}
      onConfirm={() => {
        endSeries.mutate(
          { seriesId, fromDate },
          {
            onSuccess: () => {
              onClose();
              onEnded();
            },
          },
        );
      }}
      title={t.bookings.series.endConfirmTitle}
      description={t.bookings.series.endConfirmDescription}
      confirmLabel={t.bookings.series.endConfirmLabel}
      variant="destructive"
      isLoading={endSeries.isPending}
    />
  );
}
