import { useCallback, useEffect, useRef } from 'react';
import { HTTPError } from 'ky';
import { toast } from 'sonner';
import { useQueryClient } from '@tanstack/react-query';
import type { SelectedSlot } from '../../components/CourtSelector';
import type { BookingSlotInfo } from '../../components/BookingForm';
import type { DurationMinutes, PublicComplex, PublicHoldRequest } from '../../api/types';
import { publicComplexPath } from '../../lib/publicPaths';
import { navigateTo } from '../../lib/navigation';
import { readConfirmDraft, saveConfirmDraft } from '../../lib/handoff';
import { getProblem } from '../../lib/ApiError';
import { holdRefusalCopy } from '../../lib/bookingRefusals';
import { BOOKING_MESSAGES } from '../../messages';
import { useTakeHold } from '../../hooks/useTakeHold';
import { buildConfirmState } from './buildConfirmState';
import type { HoldChallenge } from './useHoldChallenge';

const t = BOOKING_MESSAGES;

/** The body of `POST /book/holds`, with the token and the previous hold when there are any. */
function holdRequest(slotInfo: BookingSlotInfo, token?: string, previous?: string): PublicHoldRequest {
  return {
    complex_id: slotInfo.complexId,
    court_id: slotInfo.courtId,
    date: slotInfo.date,
    start_time: slotInfo.startTime,
    duration_minutes: slotInfo.durationMinutes as DurationMinutes,
    ...(token ? { turnstile_token: token } : {}),
    ...(previous ? { replaces: previous } : {}),
  };
}

/**
 * A court picked before the challenge had a token, held as soon as the token
 * arrives. Returns the function that parks the selection.
 */
function useContinueOnceSolved(token: string | undefined, takeHoldAndContinue: (selection: SelectedSlot) => void) {
  const waiting = useRef<SelectedSlot | null>(null);
  const latest = useRef(takeHoldAndContinue);
  useEffect(() => {
    latest.current = takeHoldAndContinue;
  });
  useEffect(() => {
    const selection = waiting.current;
    if (!token || !selection) return;
    waiting.current = null;
    latest.current(selection);
  }, [token]);
  return useCallback((selection: SelectedSlot) => {
    waiting.current = selection;
  }, []);
}

/**
 * Holds the slot, then off to the confirm page with it.
 *
 * The selection can be handed in directly. Choosing a court continues in the
 * same tap, and at that moment the page's `selectedSlot` is still the previous
 * render's value — reading it here would navigate with nothing. With no
 * argument, the settled selection is used.
 *
 * The hold is taken here, before the confirm page, so nobody else can book the
 * slot while this visitor types their details. It carries the Turnstile token
 * of the page's invisible widget (the one challenge of the flow) and the
 * previous hold of this tab, which the server releases first. A court picked
 * before the widget has a token waits for it instead of asking for another
 * tap. The selection and the hold travel as a draft in sessionStorage, keyed by
 * slug, because the confirm page is a separate document.
 */
export function useHandleContinue(
  slug: string | undefined,
  complex: PublicComplex | undefined,
  mpConnected: boolean,
  selectedSlot: SelectedSlot | null,
  dateStr: string,
  challenge: HoldChallenge,
) {
  const takeHold = useTakeHold();
  const queryClient = useQueryClient();
  // A second tap while the first hold is on its way would spend a token and
  // take a second hold. A ref, not `isPending`: two taps can land in one render.
  const inFlight = useRef(false);

  function takeHoldAndContinue(selection: SelectedSlot) {
    if (!complex) return;
    const pageSlug = String(slug);
    const slotInfo = buildConfirmState(complex, selection, dateStr);
    const previous = readConfirmDraft(pageSlug)?.holdToken;

    inFlight.current = true;
    takeHold.mutate(holdRequest(slotInfo, challenge.token, previous), {
      onSuccess: (hold) => {
        saveConfirmDraft(pageSlug, { ...slotInfo, holdToken: hold.token, holdExpiresAt: hold.expiresAt });
        navigateTo(`${publicComplexPath(pageSlug)}/book/confirm`);
      },
      onError: (error) => {
        toast.error(holdRefusalCopy(error));
        // Somebody else has the slot: the grid on screen is out of date.
        if (error instanceof HTTPError && getProblem(error)?.kind === 'slot-unavailable') {
          void queryClient.invalidateQueries({ queryKey: ['availability', pageSlug] });
        }
      },
      onSettled: () => {
        inFlight.current = false;
        // A Turnstile token is single use: the server spent it on this hold.
        challenge.reset();
      },
    });
  }

  const continueOnceSolved = useContinueOnceSolved(challenge.token, takeHoldAndContinue);

  return function handleContinue(selection: SelectedSlot | null = selectedSlot) {
    if (!complex || !selection || !mpConnected || inFlight.current) return;
    if (challenge.required && !challenge.token) {
      continueOnceSolved(selection);
      toast.info(t.publicBooking.holdChallengePending);
      return;
    }
    takeHoldAndContinue(selection);
  };
}
