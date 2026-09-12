import { useState } from 'react';
import { ArrowLeft, ArrowRight } from 'lucide-react';
import { Button } from '@/shared/components/ui/button';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

export function StepNavigation({
  onBack,
  onNext,
  nextDisabled,
}: {
  onBack: () => void;
  onNext: () => void;
  nextDisabled: boolean;
}) {
  // Shown only after someone actually tries. Announcing "falta una cancha"
  // before they have had a chance to add one is a complaint, not help.
  const [blocked, setBlocked] = useState(false);

  return (
    // No surface of its own, at any width — the steps above it have none
    // either, and a bordered bar under borderless content reads as a footer
    // for something that never opened. The rule above it does the separating.
    <div className="border-border-subtle flex items-center justify-between gap-3 border-t pt-5">
      <Button variant="ghost" size="sm" onClick={onBack} className="text-text-tertiary">
        <ArrowLeft className="size-4" />
        {t.common.back}
      </Button>
      {/* Enabled, and it explains itself when pressed.
          A real `disabled` attribute takes a button out of the tab order, so
          somebody on a keyboard or a screen reader reaches the end of this
          screen and the button simply is not there — no empty state elsewhere
          on the page can answer a control that cannot be reached.
          The gate itself is right: without a court there is nothing to book.
          What changes is how it says so.

          This wrapper already had `flex-col gap-1.5` around a single child —
          the room for the message was left here and never filled. */}
      <div className="flex flex-col items-end gap-1.5 sm:flex-row-reverse sm:items-center sm:gap-3">
        <Button
          onClick={() => {
            if (nextDisabled) setBlocked(true);
            else onNext();
          }}
          size="lg"
          className="w-full sm:w-auto"
        >
          {t.common.next}
          <ArrowRight className="size-4" />
        </Button>
        {blocked && nextDisabled && (
          <p role="alert" className="text-error-text text-xs">
            {t.complex.onboardingNeedsCourt}
          </p>
        )}
      </div>
    </div>
  );
}
