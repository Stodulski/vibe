import { useSyncExternalStore } from 'react';

// Nothing changes after mount, so there is nothing to subscribe to.
const subscribeNothing = () => () => undefined;

/**
 * False on the server and during hydration, true once the client is up. For
 * the few things that touch the browser at load (a map, a timer) and must not
 * change the markup the server already sent.
 */
export function useHasMounted(): boolean {
  return useSyncExternalStore(
    subscribeNothing,
    () => true,
    () => false,
  );
}
