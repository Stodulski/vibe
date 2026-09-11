import { useState } from 'react';

/**
 * Keeps the last non-empty value alive so a closing panel can finish its exit
 * animation.
 *
 * Radix keeps a closed `Dialog`/`Sheet` mounted until its exit animation ends,
 * but a panel that opens off an entity typically guards itself with
 * `if (!entity) return null` — and its parent clears that entity in the very
 * same tick it flips `open` to false (`open={!!slot}` with an `onClose` that
 * sets `slot` to `null` is the extreme case: the two are the same state). React
 * then unmounts the whole subtree immediately and the panel vanishes with no
 * animation at all.
 *
 * Holding the previous value lets the exit play against the content the reader
 * was already looking at. The panel is on its way out, so showing what it just
 * had is exactly right — and once it reopens with a real value, that value
 * takes over on the same render.
 */
export function useExitingValue<T>(value: T | null | undefined): T | null {
  const [retained, setRetained] = useState<T | null>(value ?? null);

  // Adjusting state during render rather than in an effect: an effect would
  // paint one frame of the stale value first, which is the flicker this hook
  // exists to avoid. React re-runs this component before touching the DOM.
  if (value != null && value !== retained) setRetained(value);

  return value ?? retained;
}
