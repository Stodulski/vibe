import { BOOKING_MESSAGES } from '../../messages';

/**
 * Tells someone who arrived from a waitlist email that another person took
 * that place first. They are already back on the list (the sweep re-arms them
 * once the place is taken), so the line says a new notice will follow.
 */
export function WaitlistTakenNotice() {
  return (
    <p
      role="status"
      className="border-border-subtle bg-bg-elevated text-text-secondary mb-4 rounded-xl border px-4 py-3 text-sm"
    >
      {BOOKING_MESSAGES.waitlist.takenFirst}
    </p>
  );
}
