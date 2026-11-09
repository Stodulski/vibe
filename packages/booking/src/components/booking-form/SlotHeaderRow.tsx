import { format } from 'date-fns/format';
import { es } from 'date-fns/locale/es';
import { BOOKING_MESSAGES } from '../../messages';
import { formatPrice, toDisplayDate } from '@vibe/ui';
import type { BookingSlotInfo } from './types';
import { TicketCourt, TicketStub } from '../ticket/TicketParts';

const t = BOOKING_MESSAGES;

const SPORT_LABELS: Record<string, string> = t.courts.sportTypes;
const COURT_TYPE_LABELS: Record<string, string> = t.courts.courtTypes;

interface SlotHeaderRowProps {
  slotInfo: BookingSlotInfo;
}

/**
 * Everything that was chosen, before the money, in the same order and shape
 * as the confirmed booking's ticket so the two screens read as one: the
 * venue, the day, the hours in large type, then the court with its sport and
 * type as chips and its description below. The tear line follows, and the
 * turn's price opens the money rows.
 */
export function SlotHeaderRow({ slotInfo }: SlotHeaderRowProps) {
  const dateObj = toDisplayDate(slotInfo.date);
  const courtTags = [
    slotInfo.sport && (SPORT_LABELS[slotInfo.sport] ?? slotInfo.sport),
    slotInfo.courtType && (COURT_TYPE_LABELS[slotInfo.courtType] ?? slotInfo.courtType),
  ].filter((tag): tag is string => Boolean(tag));

  return (
    <>
      <div className="space-y-6">
        <p className="text-text-inverse text-sm font-semibold">{slotInfo.complexName}</p>
        <div>
          <p className="text-text-inverse text-sm first-letter:uppercase">
            {format(dateObj, "EEEE d 'de' MMMM", { locale: es })}
          </p>
          <p className="text-text-inverse text-3xl font-bold tabular-nums">
            {`${slotInfo.startTime}\u00A0${t.publicBooking.timeRangeTo}\u00A0${slotInfo.endTime}`}
          </p>
        </div>
        <TicketCourt name={slotInfo.courtName} tags={courtTags} description={slotInfo.courtDescription} />
      </div>
      <TicketStub />
      <div className="flex justify-between gap-2 text-sm">
        <span className="text-text-inverse">{t.publicBooking.courtPrice}</span>
        <span className="text-text-inverse tabular-nums">{formatPrice(slotInfo.price)}</span>
      </div>
    </>
  );
}
