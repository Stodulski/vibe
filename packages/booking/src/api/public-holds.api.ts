import type { KyInstance } from 'ky';
import type { PublicHoldRequest } from './types';
import { parseWith, type ErrorReporter } from '../lib/apiParse';
import { holdResponseSchema } from '../schemas/publicBooking.schema';

/**
 * The storefront's slot hold, taken on the complex page and kept by the confirm
 * page. Merged into `createPublicBookingApi`. Letting a hold go is a beacon
 * (`lib/holdRelease`), not one of these: it runs while the page goes away.
 */
export function createPublicHoldsApi(client: KyInstance, report?: ErrorReporter) {
  return {
    /**
     * Holds the slot the visitor picked until the returned `expiresAt`. Not
     * idempotent: every call spends its Turnstile token, and `replaces`
     * releases the previous hold first, so a retry is a new hold.
     */
    createHold: (data: PublicHoldRequest) =>
      client
        .post('book/holds', { json: data })
        .json()
        .then(parseWith(holdResponseSchema, 'publicBookingApi.createHold', report)),

    /**
     * Takes back a hold the page asked to let go when it went away
     * (`leaveHold`): a reload or a discarded tab loading again. It neither
     * takes nor extends a hold, so the answer is an empty `204` and the page
     * keeps the deadline it has; a hold that is gone answers 409 `hold-expired`.
     */
    reclaimHold: (token: string): Promise<void> =>
      client.post(`book/holds/${encodeURIComponent(token)}/reclaim`).then(() => undefined),
  };
}
