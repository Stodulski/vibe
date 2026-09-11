import { useEffect, useRef } from 'react';

const GRACE_MS = 300;

/**
 * A Sheet/Dialog with an external, non-Radix-registered trigger (e.g. a table
 * row instead of a proper Trigger) sits behind the overlay once it opens. A
 * fast second click at the same screen position lands on the overlay, not the
 * trigger, and Radix reads it as an outside click — closing the sheet almost
 * as soon as it opened. This ignores outside-pointerdown dismissal for a
 * short grace period right after `open` flips true.
 */
export function useOutsideClickGrace(open: boolean) {
  const openedAtRef = useRef(0);

  useEffect(() => {
    if (open) openedAtRef.current = Date.now();
  }, [open]);

  return (event: { preventDefault: () => void }) => {
    if (Date.now() - openedAtRef.current < GRACE_MS) event.preventDefault();
  };
}
