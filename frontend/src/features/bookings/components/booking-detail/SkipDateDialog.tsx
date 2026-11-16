import { ConfirmDialog } from '@/shared/components/common/ConfirmDialog';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useSkipSeriesDate } from '../../hooks/useSkipSeriesDate';

const t = ES_AR;

/**
 * Confirms freeing one date of a fixed weekly booking: the date stops being
 * held and the series carries on every other week. `onClose` shuts this dialog,
 * `onSkipped` the sheet behind it.
 */
export function SkipDateDialog({
  open,
  onClose,
  onSkipped,
  complexId,
  seriesId,
  date,
}: {
  open: boolean;
  onClose: () => void;
  onSkipped: () => void;
  complexId: string;
  seriesId: string;
  /** The occurrence to free, `YYYY-MM-DD`. */
  date: string;
}) {
  const skipDate = useSkipSeriesDate(complexId);
  return (
    <ConfirmDialog
      open={open}
      onClose={onClose}
      onConfirm={() => {
        skipDate.mutate(
          { seriesId, date },
          {
            onSuccess: () => {
              onClose();
              onSkipped();
            },
          },
        );
      }}
      title={t.bookings.series.skipConfirmTitle}
      description={t.bookings.series.skipConfirmDescription}
      confirmLabel={t.bookings.series.skipConfirmLabel}
      // "Cancelar" next to "Cancelar esta fecha" would read as the same action.
      cancelLabel={t.common.back}
      variant="destructive"
      isLoading={skipDate.isPending}
    />
  );
}
