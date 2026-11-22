import { STATUS_BAR_STYLES } from '../BookingStatusBadge';

/**
 * What hours held on a court that are not a booking are drawn in, one colour
 * per kind. The event and match colours are not status colours: red, amber and
 * green already mean cancelled, pending and confirmed on this grid, and only the
 * owner's manual blocks keep the error red. A fixed booking is the exception, on
 * purpose: it is a booking in waiting and wears the confirmed colours. Every fill carries white text at 4.5:1 or better
 * (measured: grey 13.8, blue 5.9, teal over the page ground 7.3), and the
 * dashed border keeps each block 3:1 against the grid on its own.
 */
export const HELD_BLOCK_STYLES = {
  /** A fixed weekly booking whose date has no booking yet: the confirmed booking's own colours. */
  fixed: `${STATUS_BAR_STYLES.confirmed.bg} ${STATUS_BAR_STYLES.confirmed.border}`,
  /** An open-play event (canchas abiertas). */
  event: 'bg-info-border border-info-icon',
  /** A scheduled tournament match: the data teal, never a status colour. */
  match: 'bg-chart-accent/50 border-chart-accent',
  /** Hours a visitor is booking online: grey, a cell not free and not anybody's booking yet. */
  online: 'bg-bg-elevated border-text-tertiary',
} as const;
