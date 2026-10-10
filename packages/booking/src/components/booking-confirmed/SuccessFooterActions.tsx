import { Button } from '@vibe/ui';
import { BOOKING_MESSAGES } from '../../messages';
import { publicComplexPath } from '../../lib/publicPaths';
import type { BookingStatus } from '../../api/types';

const t = BOOKING_MESSAGES;

interface SuccessFooterActionsProps {
  slug: string;
  token: string | null;
  status: BookingStatus | undefined;
  /** False only when `GET /book/status` explicitly said `can_cancel: false` — hides the button entirely. */
  canCancel: boolean;
}

// "Nueva reserva" is the one filled primary action left on this screen.
// "Cancelar reserva" stays a plain neutral button, no icon and no red — red
// stays reserved for the cancel page's own confirm button, since a
// still-open cancellation window isn't a warning by itself. Stacked full
// width on mobile; side by side on desktop with "Nueva reserva" the wider
// of the two.
export function SuccessFooterActions({ slug, token, status, canCancel }: SuccessFooterActionsProps) {
  return (
    <div className="mt-4 flex w-full max-w-sm flex-col gap-3 sm:flex-row">
      <Button
        className="min-h-12 w-full rounded-xl sm:min-w-[14rem] sm:flex-1"
        onClick={() => {
          window.location.href = publicComplexPath(slug);
        }}
      >
        {t.publicBooking.makeAnother}
      </Button>
      {token && status === 'confirmed' && canCancel && (
        <Button
          variant="ghost"
          className="text-text-secondary min-h-12 w-full rounded-xl sm:w-auto"
          onClick={() => {
            window.location.href = `${publicComplexPath(slug)}/book/cancel?token=${encodeURIComponent(token)}`;
          }}
        >
          {t.publicBooking.cancelBooking}
        </Button>
      )}
    </div>
  );
}
