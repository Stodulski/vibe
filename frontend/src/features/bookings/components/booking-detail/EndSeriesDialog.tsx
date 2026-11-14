import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useEndBookingSeries } from '../../hooks/useEndBookingSeries';
const t = ES_AR;

/**
 * Confirms ending a fixed weekly booking from one date on. The dates from there
 * on stop being held; an occurrence already turned into a booking is cancelled
 * if unpaid and kept if paid, which is what the description tells the owner
 * before they confirm. `onClose` shuts this
 * dialog, `onEnded` the booking sheet behind it.
 */
export function EndSeriesDialog({
  open,
  onClose,
  onEnded,
  complexId,
  seriesId,
  fromDate,
}: {
  open: boolean;
  onClose: () => void;
  onEnded: () => void;
  complexId: string;
  /** Null for an ordinary booking: there is nothing to end. */
  seriesId: string | null | undefined;
  /** The occurrence the owner is looking at, `YYYY-MM-DD`: where the series ends. */
  fromDate: string;
}) {
  const endSeries = useEndBookingSeries(complexId);
  if (!seriesId) return null;

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
      // "Cancelar" next to "Cancelar turno fijo" would read as the same action.
      cancelLabel={t.common.back}
      variant="destructive"
      isLoading={endSeries.isPending}
    />
  );
}
