import { useState } from 'react';

/**
 * Radix's Dialog/AlertDialog/CommandDialog only restore focus to whatever
 * opened them when that element was rendered via their own `*Trigger` —
 * `context.triggerRef`, populated exclusively by the trigger component. Most
 * of this app's dialogs open through an `open`/`onClose` prop pair from an
 * ancestor instead, so that ref stays null and the built-in restore silently
 * does nothing. This hook snapshots `document.activeElement` the instant
 * `open` turns true, so the caller can restore it itself from the dialog's
 * `onCloseAutoFocus`.
 *
 * The snapshot happens during render ("adjusting state while rendering" —
 * React's own escape hatch for state that must track the exact instant a
 * prop changes, not just its current value), not in a `useEffect`: Radix's
 * own focus-trap effect (which moves focus into the dialog) runs first, so
 * an effect here would read `document.activeElement` after focus had
 * already left the trigger.
 */
export function useFocusRestoreOnClose(open: boolean | undefined): HTMLElement | null {
  const [wasOpen, setWasOpen] = useState(false);
  const [lastFocused, setLastFocused] = useState<HTMLElement | null>(null);

  if (open && !wasOpen) {
    setWasOpen(true);
    const activeElement = document.activeElement;
    setLastFocused(activeElement instanceof HTMLElement ? activeElement : null);
  } else if (!open && wasOpen) {
    setWasOpen(false);
  }

  return lastFocused;
}
