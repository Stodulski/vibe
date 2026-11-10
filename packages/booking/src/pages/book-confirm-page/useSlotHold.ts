import { useCallback, useEffect, useRef } from 'react';
import { useBookingConfig } from '../../config';
import { HOURS_ANCHOR } from '../../components/CourtSelector';
import { releaseHold } from '../../lib/holdRelease';
import { navigateTo } from '../../lib/navigation';
import { publicComplexPath } from '../../lib/publicPaths';
import { clearConfirmDraft, saveHoldExpiredNotice, type ConfirmDraft } from '../../lib/handoff';

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
 * The life of the hold the confirm page was opened with.
 *
 * Leaving gives the slot back: "Cambiar horario" releases it, and so does the
 * page going away for any other reason (the browser's back, a closed tab, a
 * reload), through a beacon on `pagehide`. The draft goes with it, so a reload
 * does not show a hold that no longer exists. Two exits keep the hold: the
 * booking that converted it (the redirect to MercadoPago must find it still
 * there), and its own expiry, which has nothing left to release.
 */
export function useSlotHold(slug: string, draft: ConfirmDraft) {
  const { apiBaseUrl } = useBookingConfig();
  // Set once the hold has been dealt with: released, converted or expired.
  // From then on the page leaving must not touch it.
  const settled = useRef(false);
  const releasedOnHide = useRef(false);
  const hours = hoursPath(slug, draft);

  const release = useCallback(() => {
    if (settled.current) return;
    settled.current = true;
    releaseHold(apiBaseUrl, draft.holdToken);
    clearConfirmDraft(slug);
  }, [apiBaseUrl, draft.holdToken, slug]);

  useEffect(() => {
    const onPageHide = () => {
      if (settled.current) return;
      releasedOnHide.current = true;
      release();
    };
    // Restored from the back/forward cache after `pagehide` released the
    // hold: the form on screen has nothing left to book, so back to the hours.
    const onPageShow = (event: PageTransitionEvent) => {
      if (event.persisted && releasedOnHide.current) navigateTo(hours, { replace: true });
    };
    window.addEventListener('pagehide', onPageHide);
    window.addEventListener('pageshow', onPageShow);
    return () => {
      window.removeEventListener('pagehide', onPageHide);
      window.removeEventListener('pageshow', onPageShow);
    };
  }, [release, hours]);

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
  }, []);

  return { changeTime, expire, markConverted };
}
