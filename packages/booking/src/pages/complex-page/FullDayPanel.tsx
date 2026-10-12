import { format } from 'date-fns/format';
import { parseISO } from 'date-fns/parseISO';
import { es } from 'date-fns/locale/es';
import { Button, WhatsappIcon } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { useNextAvailable } from '../../hooks/useNextAvailable';
import { whatsappChatHref } from '../../lib/whatsapp';
import type { DurationMinutes } from '../../api/types';
import { WaitlistForm } from './WaitlistForm';

const t = BOOKING_MESSAGES;

interface FullDayPanelProps {
  slug: string;
  /** The selected day, `YYYY-MM-DD`. */
  date: string;
  /** The chosen duration, or `null` while the visitor has not answered that question. */
  duration: DurationMinutes | null;
  phone: string;
  onSelectDate: (date: Date) => void;
}

/** "sábado 17/10": the weekday, then the day and the month, the way the copy writes them. */
function dayLabel(date: Date): string {
  return format(date, 'EEEE dd/MM', { locale: es });
}

/**
 * What a visitor sees on a day with no free slot for any sport or duration. The
 * day cannot be booked, so the panel offers the three ways forward instead: the
 * next day that can, the club on WhatsApp, and a waitlist for this one.
 */
export function FullDayPanel({ slug, date, duration, phone, onSelectDate }: FullDayPanelProps) {
  const { data: nextDate } = useNextAvailable(slug, date, duration);
  const next = nextDate ? parseISO(nextDate) : null;
  const whatsAppHref = whatsappChatHref(
    phone,
    `${t.publicBooking.whatsAppFullDayMessage} ${format(parseISO(date), 'dd/MM')}`,
  );

  return (
    <section className="border-border-interactive bg-bg-subtle animate-fade-in flex flex-col gap-6 rounded-2xl border p-5 text-left sm:p-6">
      <h3 className="text-text-primary text-lg font-semibold">{t.publicBooking.fullDayTitle}</h3>

      {next && (
        <div className="flex flex-col items-start gap-3">
          <p className="text-text-secondary text-sm">{`${t.publicBooking.nextAvailableDay} ${dayLabel(next)}`}</p>
          <Button
            onClick={() => {
              onSelectDate(next);
            }}
          >
            {`${t.publicBooking.viewNextAvailableDay} ${dayLabel(next)}`}
          </Button>
        </div>
      )}

      {whatsAppHref && (
        <Button asChild variant="outline" className="self-start">
          <a href={whatsAppHref} target="_blank" rel="noopener noreferrer">
            <WhatsappIcon className="size-4" />
            {t.publicBooking.whatsAppFullDay}
          </a>
        </Button>
      )}

      <WaitlistForm slug={slug} date={date} />
    </section>
  );
}
