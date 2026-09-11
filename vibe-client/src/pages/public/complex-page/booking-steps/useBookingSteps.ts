import { useState } from 'react';
import type { DurationMinutes, Sport } from '@/shared/types/api.types';

export type StepId = 'sport' | 'duration';

interface UseBookingStepsParams {
  /** Sports this venue actually has courts for. */
  availableSports: Sport[];
  duration: DurationMinutes;
  /**
   * Questions already answered before this mount — by the URL, when the
   * visitor comes back from the confirm page. They open as answered rather
   * than being asked again.
   */
  initiallyAnswered?: { sport: boolean; duration: boolean } | undefined;
}

/**
 * Which question the booking page is asking right now.
 *
 * Two questions come before the hours, and both earn their place by changing
 * what exists rather than by expressing a preference: a padel player cannot
 * use a tennis court, and the duration is what generates the grid of start
 * times in the first place (`grid.Slots(duration)` on the server) as well as
 * what each one costs. Court type is deliberately NOT among them — it is a
 * preference held by a minority, and asking everyone for it would also cut
 * hours out of the grid for people who never cared, with no sign of why. The
 * hours say the type themselves instead.
 *
 * A question with one possible answer is not asked. A club with a single
 * sport goes straight to the duration; the cost of the wizard is paid only by
 * venues that actually have the variety it sorts through.
 *
 * Answered steps stay reachable. Editing one re-opens that question without
 * discarding the later answers, so correcting the first step from the last
 * costs one tap rather than walking back through everything in between.
 */
export function useBookingSteps({ availableSports, duration, initiallyAnswered }: UseBookingStepsParams) {
  const [sportAnswered, setSportAnswered] = useState(initiallyAnswered?.sport ?? false);
  const [durationAnswered, setDurationAnswered] = useState(initiallyAnswered?.duration ?? false);
  const [editing, setEditing] = useState<StepId | null>(null);

  // One sport is not a choice, so it is not a question — and it counts as
  // answered from the start rather than being skipped later, which keeps
  // "which step is open" a single expression instead of a special case.
  const asksSport = availableSports.length > 1;

  const pending: StepId | null = asksSport && !sportAnswered ? 'sport' : !durationAnswered ? 'duration' : null;

  const activeStep = editing ?? pending;

  function answer(step: StepId) {
    if (step === 'sport') setSportAnswered(true);
    else setDurationAnswered(true);
    setEditing(null);
  }

  return {
    /** The question on screen, or null once every one has an answer. */
    activeStep,
    /**
     * True when the open question is one being asked for the first time, and
     * false when it was re-opened from a breadcrumb.
     *
     * The two look different on purpose. On the way in there are no hours to
     * show yet, so the question has the screen to itself. Re-opening an
     * answered question keeps the hours underneath it: changing 90 to 60 is a
     * comparison, and a comparison needs both sides — hiding the grid to ask
     * about it takes away the very thing being compared.
     */
    isFirstPass: editing === null,
    /** Steps to show as answered, in the order they were asked. */
    answered: [
      ...(asksSport && sportAnswered ? ['sport' as const] : []),
      ...(durationAnswered ? ['duration' as const] : []),
    ],
    /** True while no question is open, so the hours can be shown. */
    showsHours: activeStep === null,
    duration,
    answer,
    edit: setEditing,
  };
}
