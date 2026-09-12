import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

/**
 * The step out of the grid, in flow, directly under the choice it confirms.
 *
 * It replaces a `sticky bottom-3` bar. That bar appeared in the same gesture
 * that opened or answered the court question and then sat on top of it: at
 * 1440 it covered 6 of 11 court chips, at 320 it cut the list in half and its
 * own summary truncated to "/ 29 de ago" because the court name did not fit.
 * A control that hides the thing it refers to is worse than no control.
 *
 * And it carried a summary — court, date, hours, duration, price — that the
 * confirm page repeats at the top of the very next screen. The same facts
 * twice with no decision in between: what earns the space is the action, not
 * the recap. The selected hour and court are already marked in the grid
 * above.
 *
 * The tap stays explicit rather than navigating straight from the grid. The
 * alternative would cost two taps sometimes and three others — whether the
 * court question appears depends on data the visitor cannot see — and on a
 * phone it would turn a dense grid of hours into a field of buttons that each
 * jump to a payment form.
 */
export function ContinueAction({ onContinue }: { onContinue: () => void }) {
  return (
    <div className="mt-4">
      {/* Wrapped so the click event never reaches `onContinue`, which can
          take a selection as its argument. */}
      <Button
        onClick={() => {
          onContinue();
        }}
        className="shadow-brand h-12 w-full rounded-xl sm:w-auto sm:px-8"
      >
        {t.publicBooking.continue}
      </Button>
    </div>
  );
}
