import { getHttpErrorMessage, getHttpStatus } from '@/shared/lib/utils';

// The API's one Spanish 409 detail: a visitor is booking the hours online right
// now, and it says when they free up if nobody pays. It is already the owner's
// sentence, so it is shown as it comes.
const ONLINE_HOLD_LEAD = 'Este horario se está reservando online.';

/**
 * The server's sentence when a 409 is a visitor's live online hold on the hours
 * (it says until when), or `null` when the error is not one.
 */
export function onlineHoldMessage(error: unknown): string | null {
  if (getHttpStatus(error) !== 409) return null;
  const detail = getHttpErrorMessage(error, '').trim();
  return detail.startsWith(ONLINE_HOLD_LEAD) ? detail : null;
}
