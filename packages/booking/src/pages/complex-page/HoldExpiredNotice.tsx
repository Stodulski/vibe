import { useEffect, useState } from 'react';
import { AlertCircle } from 'lucide-react';
import { clearHoldExpiredNotice, hasHoldExpiredNotice } from '../../lib/handoff';
import { useHasMounted } from '../../hooks/useHasMounted';
import { BOOKING_MESSAGES } from '../../messages';

const t = BOOKING_MESSAGES;

/**
 * "Your time ran out", for a visitor the confirm page sent back here because
 * their hold expired. The confirm page leaves a notice in sessionStorage. It is
 * read only once the client is up (the server render has no storage, and the
 * hydrated markup must match it), and cleared once shown, so it shows once.
 */
export function HoldExpiredNotice({ slug }: { slug: string }) {
  const mounted = useHasMounted();
  return mounted ? <WaitingNotice slug={slug} /> : null;
}

function WaitingNotice({ slug }: { slug: string }) {
  const [show] = useState(() => hasHoldExpiredNotice(slug));
  useEffect(() => {
    if (show) clearHoldExpiredNotice(slug);
  }, [show, slug]);

  if (!show) return null;
  return (
    <div
      role="alert"
      className="border-warning-border bg-warning-bg text-text-primary flex items-start gap-2 rounded-xl border p-4 text-sm"
    >
      <AlertCircle className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
      {t.publicBooking.holdExpired}
    </div>
  );
}
