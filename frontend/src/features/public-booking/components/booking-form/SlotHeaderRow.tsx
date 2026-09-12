import { format } from 'date-fns/format';
import { es } from 'date-fns/locale/es';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice, toDisplayDate } from '@/shared/lib/utils';
import type { BookingSlotInfo } from './types';

const t = ES_AR;

const SPORT_LABELS: Record<string, string> = t.courts.sportTypes;
const COURT_TYPE_LABELS: Record<string, string> = t.courts.courtTypes;

interface SlotHeaderRowProps {
  slotInfo: BookingSlotInfo;
}

/**
 * Everything that was chosen, before the money, in the same order and shape
 * as the confirmed booking's ticket so the two screens read as one: the
 * venue, the day, the hours in large type, then the court with its sport and
 * type on their own lines and the court's description. Each group sits in
 * its own band separated by a hairline; the turn's price closes the block as
 * the first money row.
 */
export function SlotHeaderRow({ slotInfo }: SlotHeaderRowProps) {
  const dateObj = toDisplayDate(slotInfo.date);
  const courtLines = [
    slotInfo.sport && (SPORT_LABELS[slotInfo.sport] ?? slotInfo.sport),
    slotInfo.courtType && (COURT_TYPE_LABELS[slotInfo.courtType] ?? slotInfo.courtType),
    slotInfo.courtDescription,
  ].filter((line): line is string => Boolean(line));

  return (
    <div className="divide-border-subtle divide-y">
      <p className="text-text-primary pb-3 text-sm font-semibold">{slotInfo.complexName}</p>
      <div className="py-3">
        <p className="text-text-secondary text-sm first-letter:uppercase">
          {format(dateObj, "EEEE d 'de' MMMM", { locale: es })}
        </p>
        <p className="text-text-primary text-3xl font-bold tabular-nums">
          {`${slotInfo.startTime}\u00A0${t.publicBooking.timeRangeTo}\u00A0${slotInfo.endTime}`}
        </p>
      </div>
      <div className="space-y-1 py-3">
        <p className="text-text-primary text-base font-semibold">{slotInfo.courtName}</p>
        {courtLines.map((line) => (
          <p key={line} className="text-text-secondary text-sm">
            {line}
          </p>
        ))}
      </div>
      <div className="flex justify-between gap-2 py-3 text-sm">
        <span className="text-text-secondary">{t.publicBooking.courtPrice}</span>
        <span className="text-text-primary tabular-nums">{formatPrice(slotInfo.price)}</span>
      </div>
    </div>
  );
}
