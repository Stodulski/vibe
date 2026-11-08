import { MapPin, Phone, Mail } from "lucide-react";
import { Button, InstagramIcon, WhatsappIcon } from "@vibe/ui";
import { BOOKING_MESSAGES } from "../../messages";
import type { PublicComplex } from "../../api/types";
import { fullAddress } from "../../lib/complexAddress";
import { instagramProfileUrl } from "../../lib/instagram";
import { whatsappChatHref } from "../../lib/whatsapp";

const t = BOOKING_MESSAGES;

/** The club's Instagram profile, opened in a new tab, on the same row rhythm as the phone and email. */
function InstagramLink({ name, handle }: { name: string; handle: string }) {
  return (
    <a
      href={instagramProfileUrl(handle)}
      target="_blank"
      rel="noopener noreferrer"
      aria-label={`${t.publicBooking.instagramOf} ${name}: @${handle}`}
      className="hover:text-primary-400 flex w-fit items-center gap-2 transition-colors pointer-coarse:-my-[15px] pointer-coarse:min-h-0 pointer-coarse:py-[15px]"
    >
      <InstagramIcon className="text-primary-400 size-3 shrink-0 sm:size-3.5" />
      <span>@{handle}</span>
    </a>
  );
}

/**
 * The club's WhatsApp chat for the same number as the phone, opened in a new
 * tab. A real button under the contact rows: as one more row it read as a
 * label, not as something to press.
 */
function WhatsAppButton({ name, href }: { name: string; href: string }) {
  return (
    <Button asChild variant="outline" className="mt-4 w-full sm:w-fit">
      <a
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        aria-label={`${BOOKING_MESSAGES.publicBooking.writeOnWhatsAppTo} ${name}`}
      >
        <WhatsappIcon className="text-primary-400 size-4" />
        {BOOKING_MESSAGES.publicBooking.whatsAppLabel}
      </a>
    </Button>
  );
}

/**
 * How to find the club and how to reach it: address, phone, email and
 * Instagram, one per row, then a WhatsApp button. A detail the club has not
 * filled in is left out rather than shown as "Sin email".
 *
 * A column, not a row. These were laid out side by side, which reads as one
 * run-on line of unrelated facts and gives each of them whatever width is
 * left over — the address was `truncate`d for exactly that reason, hiding the
 * end of a street name behind a hover that does not exist on a phone. Stacked,
 * each fact is one line, they align down a single edge, and adding the email
 * costs nothing.
 *
 * Today's opening hours used to sit here as a fourth item. They moved into
 * the full week below, where they can be read against the other days.
 *
 * Phone and email are links, not text: the point of showing them is that
 * someone acts on them. On a touch screen every link gets a 44px minimum
 * height from the global stylesheet, which made these two rows taller than
 * the address above them and the spacing uneven. The links keep the 44px
 * tap area as padding and take it back as negative margin, so the three
 * rows sit on the same 14px rhythm.
 */
export function ComplexInfoRow({ complex }: { complex: PublicComplex }) {
  const whatsAppHref = whatsappChatHref(complex.phone);

  return (
    <div className="text-micro text-text-secondary mt-3 flex flex-col gap-2 sm:text-xs lg:gap-2.5 lg:text-sm">
      {/* All three rows centre their icon against the text. The address used
          `items-start` with a compensating `mt-0.5`, which put its pin two
          pixels lower than the phone's and the mail's — the rows are evenly
          spaced, but three icons on two different baselines read as uneven. */}
      <div className="flex items-center gap-2">
        <MapPin
          className="text-primary-400 size-3 shrink-0 sm:size-3.5"
          aria-hidden="true"
        />
        <span>{fullAddress(complex)}</span>
      </div>

      {complex.phone && (
        <a
          href={`tel:${complex.phone}`}
          className="hover:text-primary-400 flex w-fit items-center gap-2 transition-colors pointer-coarse:-my-[15px] pointer-coarse:min-h-0 pointer-coarse:py-[15px]"
        >
          <Phone
            className="text-primary-400 size-3 shrink-0 sm:size-3.5"
            aria-hidden="true"
          />
          <span>{complex.phone}</span>
        </a>
      )}

      {complex.email && (
        <a
          href={`mailto:${complex.email}`}
          className="hover:text-primary-400 flex w-fit items-center gap-2 transition-colors pointer-coarse:-my-[15px] pointer-coarse:min-h-0 pointer-coarse:py-[15px]"
        >
          <Mail
            className="text-primary-400 size-3 shrink-0 sm:size-3.5"
            aria-hidden="true"
          />
          <span>{complex.email}</span>
        </a>
      )}

      {complex.instagram && (
        <InstagramLink name={complex.name} handle={complex.instagram} />
      )}

      {/* Without online booking, the booking panel below already offers this
          chat as its main button; a second one up here would compete. */}
      {whatsAppHref && complex.payments_enabled && (
        <WhatsAppButton name={complex.name} href={whatsAppHref} />
      )}
    </div>
  );
}
