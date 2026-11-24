import { useCallback, useEffect, useRef, type RefObject } from 'react';
import { useBookingConfig } from '../../config';
import { HOURS_ANCHOR } from '../../components/CourtSelector';
import { leaveHold, releaseHold } from '../../lib/holdRelease';
import { getProblem } from '../../lib/ApiError';
import { HOLD_EXPIRED_KIND } from '../../lib/bookingRefusals';
import { usePublicBookingApi } from '../../api/context';
import { navigateTo } from '../../lib/navigation';
import { publicComplexPath } from '../../lib/publicPaths';
import { clearConfirmDraft, readConfirmDraft, saveHoldExpiredNotice, type ConfirmDraft } from '../../lib/handoff';

/**
 * Where "Cambiar horario" and an expired hold land: the hours of the complex
 * page. The query keeps the date, duration and sport already answered, the
 * hour is left out so the grid shows instead of the court question, and
 * `#horarios` tells the grid to bring itself into view (`CourtSelector`).
 */
export function hoursPath(slug: string, draft: ConfirmDraft): string {
  const query = new URLSearchParams({ date: draft.date, duration: String(draft.durationMinutes) });
  if (draft.sport) query.set('sport', draft.sport);
  return `${publicComplexPath(slug)}?${query.toString()}${HOURS_ANCHOR}`;
}

/**
 * Whether a failed reclaim says the hold is gone, rather than that it could not
 * be asked about. Only the server's `hold-expired`: a 404 is a server without
 * the reclaim route yet, which must not send a visitor with a live hold away.
 */
function isHoldGone(error: unknown): boolean {
  const problem = getProblem(error);
  return problem?.status === 409 && problem.kind === HOLD_EXPIRED_KIND;
}

/**
 * How long after taking the hold back the page asks again. The leave beacon of
 * the document a reload replaced (or of a duplicated tab closing) can reach the
 * server after this page's first reclaim and ask for the release once more;
 * the second ask calls it off well within the minute's grace.
 */
export const REASSERT_HOLD_AFTER_MS = 10_000;

interface HoldRefs {
  settledRef: RefObject<boolean>;
  leftOnHideRef: RefObject<boolean>;
  submittingRef: RefObject<boolean>;
}

/**
 * Takes the hold back from a release the page's leaving asked for: when the
 * page loads with its draft (a first visit, a reload, a tab the phone
 * discarded), when the visitor returns to the tab, whenever the returned
 * function is called, and once more `REASSERT_HOLD_AFTER_MS` after each of
 * those. A hold that is gone settles the page and sends the visitor to the
 * hours; one that could not be asked about (offline, a server error) leaves
 * the page as it is, and the submit says so if the hold is gone after all.
 * Nothing is asked once the hold is settled or while the booking is being
 * submitted, whose answer is the one that counts.
 */
function useReclaimHold(slug: string, holdToken: string, hours: string, refs: HoldRefs) {
  const { settledRef, leftOnHideRef, submittingRef } = refs;
  const api = usePublicBookingApi();
  const reassertRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  const ask = useCallback(() => {
    if (settledRef.current || submittingRef.current) return;
    api.reclaimHold(holdToken).catch((error: unknown) => {
      if (settledRef.current || submittingRef.current || !isHoldGone(error)) return;
      settledRef.current = true;
      clearConfirmDraft(slug);
      navigateTo(hours, { replace: true });
    });
  }, [api, holdToken, slug, hours, settledRef, submittingRef]);

  const reclaim = useCallback(() => {
    leftOnHideRef.current = false;
    ask();
    clearTimeout(reassertRef.current);
    reassertRef.current = setTimeout(ask, REASSERT_HOLD_AFTER_MS);
  }, [ask, leftOnHideRef]);

  useEffect(() => {
    reclaim();
    const onVisibilityChange = () => {
      if (document.visibilityState === 'visible') reclaim();
    };
    document.addEventListener('visibilitychange', onVisibilityChange);
    return () => {
      document.removeEventListener('visibilitychange', onVisibilityChange);
      clearTimeout(reassertRef.current);
    };
  }, [reclaim]);

  return reclaim;
}

/**
 * The life of the hold the confirm page was opened with.
 *
 * "Cambiar horario" gives the slot back at once: the visitor chose another
 * time. The page going away for any other reason (the browser's back, a
 * closed tab, a reload, a phone discarding the background tab) only asks the
 * server, through a beacon on `pagehide`, to let it go a minute later, and the
 * draft stays. When the page loads again with that draft, or the browser shows
 * it again from its back/forward cache, it takes the hold back
 * (`reclaimHold`): the countdown carries on from the deadline already stored,
 * since a reclaim never extends the hold. A hold that is gone by then sends the
 * visitor to the hours. Two exits keep the hold: the booking that converted it
 * (the redirect to MercadoPago must find it still there), and its own expiry,
 * which has nothing left to release.
 *
 * A page restored from the back/forward cache comes back as it was left, so it
 * asks the draft which hold is current first: another one (held on the
 * complex page meanwhile) replaces the page's through `onDraftReplaced`; none
 * sends the visitor to the hours. A converted hold is the submit's to follow.
 */
export function useSlotHold(slug: string, draft: ConfirmDraft, onDraftReplaced: (draft: ConfirmDraft) => void) {
  const { apiBaseUrl } = useBookingConfig();
  // Set once the hold has been dealt with: released, converted or expired.
  // From then on the page leaving must not touch it.
  const settled = useRef(false);
  const leftOnHide = useRef(false);
  const converted = useRef(false);
  const submitting = useRef(false);
  const hours = hoursPath(slug, draft);

  const release = useCallback(() => {
    if (settled.current) return;
    settled.current = true;
    releaseHold(apiBaseUrl, draft.holdToken);
    clearConfirmDraft(slug);
  }, [apiBaseUrl, draft.holdToken, slug]);

  const refs = { settledRef: settled, leftOnHideRef: leftOnHide, submittingRef: submitting };
  const reclaim = useReclaimHold(slug, draft.holdToken, hours, refs);

  useEffect(() => {
    const onPageHide = () => {
      if (settled.current) return;
      leftOnHide.current = true;
      leaveHold(apiBaseUrl, draft.holdToken);
    };
    // Restored from the back/forward cache: the hold on screen may be gone.
    const onPageShow = (event: PageTransitionEvent) => {
      if (!event.persisted || converted.current || settled.current) return;
      const current = readConfirmDraft(slug);
      if (current && current.holdToken !== draft.holdToken) {
        onDraftReplaced(current);
      } else if (!current) {
        navigateTo(hours, { replace: true });
      } else if (leftOnHide.current) {
        reclaim();
      }
    };
    window.addEventListener('pagehide', onPageHide);
    window.addEventListener('pageshow', onPageShow);
    return () => {
      window.removeEventListener('pagehide', onPageHide);
      window.removeEventListener('pageshow', onPageShow);
    };
  }, [apiBaseUrl, reclaim, hours, slug, draft.holdToken, onDraftReplaced]);

  /** "Cambiar horario": the slot goes back, the visitor goes to the hours. */
  const changeTime = useCallback(() => {
    release();
    navigateTo(hours, { replace: true });
  }, [release, hours]);

  /** The time is up (on screen, or as the server's answer): say so on the hours. */
  const expire = useCallback(() => {
    settled.current = true;
    clearConfirmDraft(slug);
    saveHoldExpiredNotice(slug);
    navigateTo(hours, { replace: true });
  }, [slug, hours]);

  /** The booking converted the hold. Leaving now, MercadoPago included, keeps it. */
  const markConverted = useCallback(() => {
    settled.current = true;
    converted.current = true;
  }, []);

  /** The booking is being submitted, or no longer is: meanwhile its answer, not a reclaim's, decides. */
  const markSubmitting = useCallback((active: boolean) => {
    submitting.current = active;
  }, []);

  return { changeTime, expire, markConverted, markSubmitting };
}
