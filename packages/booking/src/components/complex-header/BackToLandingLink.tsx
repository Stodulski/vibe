import type { MouseEvent } from 'react';
import { ArrowLeft } from 'lucide-react';
import { useHasMounted } from '../../hooks/useHasMounted';
import { landingReferrerPath } from '../../lib/landingReferrer';
import { BOOKING_MESSAGES } from '../../messages';

const t = BOOKING_MESSAGES;

/**
 * "Volver", shown only to a visitor who came from a page of the landing (a city
 * directory, the home page). It goes back in history, so the directory reopens
 * where it was scrolled; a tab with no history follows the link instead.
 *
 * The referrer is read after mount: the server render has none, and the markup
 * it sent must match the first client render.
 */
export function BackToLandingLink() {
  const mounted = useHasMounted();
  if (!mounted) return null;

  const path = landingReferrerPath(document.referrer, window.location.origin);
  if (!path) return null;

  function goBack(event: MouseEvent<HTMLAnchorElement>) {
    if (window.history.length <= 1) return;
    event.preventDefault();
    window.history.back();
  }

  return (
    <a
      href={path}
      onClick={goBack}
      className="text-text-secondary mb-3 flex min-h-11 w-fit items-center gap-2 text-sm transition-opacity hover:opacity-70"
    >
      <ArrowLeft className="size-4 shrink-0" aria-hidden="true" />
      <span>{t.publicBooking.backToLanding}</span>
    </a>
  );
}
