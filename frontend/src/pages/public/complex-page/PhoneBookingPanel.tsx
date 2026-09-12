import { Button } from '@/shared/components/ui/button';
import { WhatsappIcon } from '@/shared/components/common/WhatsappIcon';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * What a complex takes bookings by, when it cannot take them online.
 *
 * This replaces two things that were on the page together and contradicted
 * each other: the full availability grid with every slot disabled, and a
 * warning banner saying online booking was unavailable. A visitor scrolled
 * through hundreds of prices, found every one of them unclickable, and left
 * without learning that the club has a phone.
 *
 * The number is the button rather than a line of text above one, and it
 * opens WhatsApp rather than dialling: a chat is how a court is actually
 * booked with a club in Argentina, and unlike `tel:` it works on a desktop
 * too. The message is prefilled so the visitor lands with the question
 * already asked.
 */
export function PhoneBookingPanel({ phone }: { phone: string }) {
  const digits = phone.replace(/\D/g, '');
  const href = `https://wa.me/${digits}?text=${encodeURIComponent(t.publicBooking.phoneOnlyMessage)}`;
  return (
    <section className="lg:border-border-subtle lg:bg-bg-subtle flex flex-col items-start gap-4 py-10 text-left lg:rounded-2xl lg:border lg:px-6">
      <div className="space-y-2">
        <h2 className="text-text-primary text-lg font-semibold">{t.publicBooking.phoneOnlyTitle}</h2>
        <p className="text-text-secondary max-w-sm text-sm">{t.publicBooking.phoneOnlyDescription}</p>
      </div>
      <Button asChild size="lg">
        <a
          href={href}
          target="_blank"
          rel="noopener noreferrer"
          aria-label={`${t.publicBooking.phoneOnlyCallLabel}: ${phone}`}
        >
          <WhatsappIcon className="size-4" />
          {phone}
        </a>
      </Button>
    </section>
  );
}
