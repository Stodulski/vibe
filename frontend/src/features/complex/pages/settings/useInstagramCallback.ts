import { useEffect } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useQueryClient } from '@tanstack/react-query';
import { toast } from 'sonner';
import { queryKeys } from '@/shared/lib/queryKeys';
import { ES_AR } from '@/shared/i18n/es_AR';

const t = ES_AR;

const ERROR_MESSAGES: Record<string, string> = t.instagram.callbackErrors;

// One id for every callback toast: StrictMode runs the effect twice before
// the cleaned URL lands, and sonner replaces a toast with the same id instead
// of stacking a second copy.
const TOAST_ID = 'instagram-callback';

/**
 * Shows the outcome the Instagram OAuth callback put in the URL
 * (`?instagram=connected`, or `?instagram=error&reason=<expired|denied|failed|unavailable>`)
 * and then drops those params, so a reload or a shared link does not repeat
 * it. `?tab=` stays: the owner lands on the Instagram tab.
 *
 * Lives on the page, not in the tab: a callback with `reason=unavailable` can
 * arrive while the feature is off and the tab is hidden, and still has to say
 * what happened.
 */
export function useInstagramCallback(complexId: string | undefined) {
  const [searchParams, setSearchParams] = useSearchParams();
  const queryClient = useQueryClient();
  const outcome = searchParams.get('instagram');
  const reason = searchParams.get('reason');

  useEffect(() => {
    if (!outcome) return;
    if (outcome === 'connected') {
      toast.success(t.instagram.connectSuccess, { id: TOAST_ID });
      if (complexId) {
        void queryClient.invalidateQueries({ queryKey: queryKeys.complexes.instagramStatus(complexId) });
      }
    } else {
      toast.error((reason && ERROR_MESSAGES[reason]) ?? t.instagram.callbackErrors.failed, { id: TOAST_ID });
    }
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        next.delete('instagram');
        next.delete('reason');
        return next;
      },
      { replace: true },
    );
  }, [outcome, reason, complexId, queryClient, setSearchParams]);
}
