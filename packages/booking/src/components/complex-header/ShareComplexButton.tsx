import { useContext, useEffect, useState } from 'react';
import { Share2 } from 'lucide-react';
import { BookingConfigContext } from '../../config';
import { publicComplexPath } from '../../lib/publicPaths';
import { browserShareEnvironment, shareComplexPage } from '../../lib/shareComplex';
import { BOOKING_MESSAGES } from '../../messages';

const t = BOOKING_MESSAGES;

/** How long "Link copiado" stays on screen. */
const COPIED_FEEDBACK_MS = 2000;

interface ShareComplexButtonProps {
  name: string;
  slug: string;
}

/**
 * "Compartir" in the club's header. It shares the club's own page and nothing else.
 *
 * The link is built on click from the storefront origin in the booking config, so the
 * server markup reads no `window` and needs no URL. The feedback is a live region that
 * is always rendered, so a screen reader announces "Link copiado" when it appears.
 */
export function ShareComplexButton({ name, slug }: ShareComplexButtonProps) {
  // Optional: the header also renders on its own (tests, previews), without a BookingRoot.
  const publicSiteUrl = useContext(BookingConfigContext)?.publicSiteUrl;
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return;
    const timer = window.setTimeout(() => {
      setCopied(false);
    }, COPIED_FEEDBACK_MS);
    return () => {
      window.clearTimeout(timer);
    };
  }, [copied]);

  async function share() {
    const origin = publicSiteUrl ?? window.location.origin;
    const url = new URL(publicComplexPath(slug), origin).toString();
    const outcome = await shareComplexPage(
      { title: name, text: `${t.publicBooking.shareTextPrefix} ${name}`, url },
      browserShareEnvironment(),
    );
    setCopied(outcome === 'copied');
  }

  return (
    <div className="text-micro text-text-secondary flex flex-wrap items-center gap-y-2 sm:text-xs lg:text-sm">
      {/* The feedback comes first: the button sits at the right edge of the header,
          so "Link copiado" appears to its left and the button does not move. */}
      <span role="status" aria-live="polite" className="text-primary-400 not-empty:mr-3">
        {copied ? t.publicBooking.shareCopied : ''}
      </span>
      {/* Drawn as one more contact row, not a button: an icon and a word, on the
          rows' own size and colour. On a touch screen it keeps the 44px tap area
          as padding and takes it back as negative margin, like the phone link. */}
      <button
        type="button"
        className="hover:text-primary-400 flex w-fit cursor-pointer items-center gap-2 transition-colors pointer-coarse:-my-[15px] pointer-coarse:min-h-0 pointer-coarse:py-[15px]"
        aria-label={`${t.publicBooking.shareAction} ${name}`}
        onClick={() => void share()}
      >
        <Share2 className="text-primary-400 size-3 shrink-0 sm:size-3.5" aria-hidden="true" />
        <span>{t.publicBooking.shareAction}</span>
      </button>
    </div>
  );
}
