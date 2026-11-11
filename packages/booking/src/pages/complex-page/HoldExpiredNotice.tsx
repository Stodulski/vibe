import { useEffect, useState } from 'react';
import { Clock } from 'lucide-react';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogTitle,
} from '@vibe/ui';
import { clearHoldExpiredNotice, hasHoldExpiredNotice } from '../../lib/handoff';
import { useHasMounted } from '../../hooks/useHasMounted';
import { BOOKING_MESSAGES } from '../../messages';

const t = BOOKING_MESSAGES;

/**
 * "Your time ran out", for a visitor the confirm page sent back here because
 * their hold expired. The confirm page leaves a notice in sessionStorage. It is
 * read only once the client is up (the server render has no storage, and the
 * hydrated markup must match it), and cleared once shown, so it shows once.
 *
 * A modal rather than an inline banner: the visitor lands on the page expecting
 * their booking, so the reason it is gone has to be read and acknowledged
 * before they pick a time again.
 */
export function HoldExpiredNotice({ slug }: { slug: string }) {
  const mounted = useHasMounted();
  return mounted ? <WaitingNotice slug={slug} /> : null;
}

function WaitingNotice({ slug }: { slug: string }) {
  const [open, setOpen] = useState(() => hasHoldExpiredNotice(slug));
  useEffect(() => {
    if (open) clearHoldExpiredNotice(slug);
  }, [open, slug]);

  return (
    <AlertDialog open={open} onOpenChange={setOpen}>
      <AlertDialogContent className="bg-bg-subtle border-border-subtle rounded-2xl border p-6 shadow-lg sm:max-w-md">
        <div className="flex flex-col items-center gap-3 text-center">
          <span className="bg-warning-bg text-warning-text flex size-11 items-center justify-center rounded-full">
            <Clock className="size-5" aria-hidden="true" />
          </span>
          <AlertDialogTitle className="text-text-primary text-lg font-semibold">
            {t.publicBooking.holdExpiredTitle}
          </AlertDialogTitle>
          <AlertDialogDescription className="text-text-secondary text-sm">
            {t.publicBooking.holdExpired}
          </AlertDialogDescription>
        </div>
        <AlertDialogAction className="mt-2 h-12 w-full">{t.publicBooking.holdExpiredAction}</AlertDialogAction>
      </AlertDialogContent>
    </AlertDialog>
  );
}
