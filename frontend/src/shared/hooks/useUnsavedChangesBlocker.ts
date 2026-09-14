import { useBlocker } from 'react-router-dom';
import { useUnsavedWork } from '@/shared/hooks/useUnsavedWork';

/**
 * What a form needs to render a "you have unsaved changes" confirmation.
 *
 * `isBlocked` is true while a navigation is waiting on an answer; `leave`
 * lets it through and `stay` cancels it and leaves the person on the form,
 * with everything they typed still there.
 */
export interface UnsavedChangesBlocker {
  isBlocked: boolean;
  leave: () => void;
  stay: () => void;
}

/**
 * Holds an in-app navigation until the person says the unsaved changes can go.
 *
 * Filling in a complex, a court or a week of opening hours takes real effort,
 * and until this existed a mis-clicked sidebar link threw all of it away
 * without a word — the form simply unmounted. React Router's `useBlocker`
 * intercepts the navigation *before* the unmount, so the form's state is still
 * there to go back to when the answer is "stay".
 *
 * Only a change of route is blocked: a `?tab=` or a `#section` on the page the
 * form is already on is not leaving the form, and confirming it would be a
 * dialog about nothing.
 *
 * This covers navigations the router owns. Closing the tab or hitting reload
 * is a different mechanism (`beforeunload`) with its own tradeoffs, and is
 * deliberately not handled here.
 *
 * It also publishes the dirty state through `useUnsavedWork`, the registry the
 * PWA update policy reads before applying a new build (see
 * `serviceWorkerUpdate.ts`). A reload the app decides on its own would destroy
 * exactly the work this hook exists to protect, so every form that guards
 * navigation guards the update too, for free.
 */
export function useUnsavedChangesBlocker(isDirty: boolean): UnsavedChangesBlocker {
  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) => isDirty && currentLocation.pathname !== nextLocation.pathname,
  );

  useUnsavedWork(isDirty);

  return {
    isBlocked: blocker.state === 'blocked',
    leave: () => {
      blocker.proceed?.();
    },
    stay: () => {
      blocker.reset?.();
    },
  };
}
