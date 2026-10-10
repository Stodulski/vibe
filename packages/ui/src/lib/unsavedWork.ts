/**
 * A registry of the forms that currently hold work nobody has saved yet.
 *
 * The service worker update policy needs one question answered — "is anyone
 * mid-task right now?" — from places that are not React: a `visibilitychange`
 * listener in `serviceWorkerUpdate.ts`, and an effect that must read the
 * answer synchronously rather than after a render. A module-level set answers
 * it without a store, a context or a provider, and stays framework-free.
 *
 * The set holds ids rather than a counter so that a double `markUnsavedWork`
 * for the same form (a re-run effect, React StrictMode) cannot leave the app
 * permanently "dirty" after that form unmounts.
 *
 * `useUnsavedChangesBlocker` is the only caller: every form that already
 * guards navigation against losing what was typed therefore guards the update
 * too, and no form has to remember to opt in twice.
 */
const unsavedIds = new Set<string>();

/** Records, or clears, that the form behind `id` has unsaved changes. */
export function markUnsavedWork(id: string, dirty: boolean): void {
  if (dirty) {
    unsavedIds.add(id);
    return;
  }
  unsavedIds.delete(id);
}

/** True while any mounted form holds changes the person has not saved. */
export function hasUnsavedWork(): boolean {
  return unsavedIds.size > 0;
}
