/**
 * Centralized app constants — business limits that may become
 * configurable per-complex in the future.
 */

/** Maximum bookings shown in dashboard "Today" widget (mobile) */
export const MAX_VISIBLE_BOOKINGS = 5;

/** Maximum bookings shown in dashboard "Today" widget (desktop — more vertical room) */
export const MAX_VISIBLE_BOOKINGS_DESKTOP = 8;

/** Default page size for paginated client list */
export const CLIENTS_PAGE_SIZE = 50;

/** Phone country prefix — owned by `@vibe/ui`, re-exported here for the app's existing imports. */
export { DEFAULT_PHONE_PREFIX } from '@vibe/ui/lib/constants.ts';
