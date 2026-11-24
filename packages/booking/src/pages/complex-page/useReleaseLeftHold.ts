import { useEffect } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { useBookingConfig } from '../../config';
import { clearConfirmDraft, readConfirmDraft } from '../../lib/handoff';
import { releaseHoldNow } from '../../lib/holdRelease';
import { queryKeys } from '../../lib/queryKeys';

/**
 * Gives back at once the hold of a confirm page the visitor came back from.
 *
 * Leaving the confirm page only asks the server to let its hold go a minute
 * later, so a reload can take it back. A visitor back on this page has left the
 * confirm step for real, though, and would otherwise see their own slot taken
 * for that minute. So a hold still in the draft when this page loads, or when
 * the browser shows it again from its back/forward cache, is released now
 * (`/release`, as "Cambiar horario" does), the draft goes with it, and the hours
 * are fetched again once the server has freed it.
 *
 * Only a draft found at those two moments: the one this page writes when the
 * court is picked is for the confirm page it is navigating to, and nothing
 * reads it here before the browser leaves. A confirm page restored afterwards
 * finds no draft and sends the visitor to the hours.
 */
export function useReleaseLeftHold(slug: string): void {
  const { apiBaseUrl } = useBookingConfig();
  const queryClient = useQueryClient();

  useEffect(() => {
    const releaseLeftHold = () => {
      const draft = readConfirmDraft(slug);
      if (!draft) return;
      clearConfirmDraft(slug);
      void releaseHoldNow(apiBaseUrl, draft.holdToken).then(() =>
        queryClient.invalidateQueries({ queryKey: queryKeys.availability.bySlug(slug) }),
      );
    };
    const onPageShow = (event: PageTransitionEvent) => {
      if (event.persisted) releaseLeftHold();
    };
    releaseLeftHold();
    window.addEventListener('pageshow', onPageShow);
    return () => {
      window.removeEventListener('pageshow', onPageShow);
    };
  }, [slug, apiBaseUrl, queryClient]);
}
