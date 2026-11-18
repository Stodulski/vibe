import { format } from 'date-fns/format';
import { es } from 'date-fns/locale/es';
import { BOOKING_MESSAGES } from '../../messages';
import { toDisplayDate } from '@vibe/ui';
import type { BookingSlotInfo } from './types';
import { TicketCourt, TicketPriceRow, TicketStub } from '../ticket/TicketParts';
import { keepTogether } from '../../lib/keepTogether';

const t = BOOKING_MESSAGES;

const SPORT_LABELS: Record<string, string> = t.courts.sportTypes;
const COURT_TYPE_LABELS: Record<string, string> = t.courts.courtTypes;

interface SlotHeaderRowProps {
  slotInfo: BookingSlotInfo;
  /** Inside a band that already is the card (see TicketParts). */
  bare?: boolean;
}

/**
 * Everything that was chosen, before the money, in the same order and shape
 * as the confirmed booking's ticket so the two screens read as one: the
 * venue, the day, the hours in large type, then the court with its sport and
 * type as chips. The tear line follows, and the
 * turn's price opens the money rows.
 */
export function SlotHeaderRow({ slotInfo, bare = false }: SlotHeaderRowProps) {
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
        <TicketCourt name={keepTogether(slotInfo.courtName)} tags={courtTags} bare={bare} />
      </div>
      <TicketStub bare={bare} />
      <TicketPriceRow label={t.publicBooking.courtPrice} amount={slotInfo.price} />
    </>
  );
}
