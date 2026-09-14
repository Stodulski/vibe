import { useEffect, useId } from 'react';
import { markUnsavedWork } from '@/shared/lib/unsavedWork';

/**
 * Declares, for as long as this component is mounted, whether it is holding
 * work a reload would destroy.
 *
 * The PWA applies a waiting build on its own at moments that are meant to cost
 * nothing — an in-app navigation, the tab going to the background — and this
 * is how a screen says "not now" (see `serviceWorkerUpdate.ts`). Anything a
 * person has put in that only lives in memory qualifies: a form they are
 * filling in, a slot they have picked but not confirmed.
 *
 * State the URL carries does not qualify: a reload lands on the same query and
 * the screen rebuilds itself from it.
 *
 * The id comes from `useId`, so it is stable for the life of the component and
 * distinct per instance — two forms on one screen cannot cancel each other's
 * registration. The cleanup is what makes unmounting safe: the work is gone
 * with the component, and the registry must not keep claiming the app is busy.
 */
export function useUnsavedWork(isDirty: boolean): void {
  const id = useId();

  useEffect(() => {
    markUnsavedWork(id, isDirty);
    return () => {
      markUnsavedWork(id, false);
    };
  }, [id, isDirty]);
}
