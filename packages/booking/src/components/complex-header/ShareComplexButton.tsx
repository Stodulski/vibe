import { useContext, useEffect, useState } from 'react';
import { Share2 } from 'lucide-react';
import { Button } from '@vibe/ui';
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
    <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
      <Button
        type="button"
        variant="outline"
        className="h-11"
        aria-label={`${t.publicBooking.shareAction} ${name}`}
        onClick={() => void share()}
      >
        <Share2 className="size-4" aria-hidden="true" />
        {t.publicBooking.shareAction}
      </Button>
      <span role="status" aria-live="polite" className="text-primary-400 text-sm">
        {copied ? t.publicBooking.shareCopied : ''}
      </span>
    </div>
  );
}
