import type { KyInstance } from 'ky';
import { parseWith, type ErrorReporter } from '../lib/apiParse';
import { checkoutRetryResponseSchema } from '../schemas/publicBooking.schema';

/**
 * What a visitor whose payment was rejected can do with the booking it left
 * behind, by the link token the failure redirect carries. Merged into
 * `createPublicBookingApi`.
 */
export function createPublicCheckoutApi(client: KyInstance, report?: ErrorReporter) {
  return {
    /** The booking's same checkout, while its window lasts (409 once it cannot be paid). */
    retryCheckout: (token: string) =>
      client
        .post('book/checkout/retry', { json: { token } })
        .json()
        .then(parseWith(checkoutRetryResponseSchema, 'publicBookingApi.retryCheckout', report)),

    /** Gives the slot back (`204`; 409 when it was paid after all). */
    releaseCheckout: (token: string) => client.post('book/checkout/release', { json: { token } }).then(() => undefined),
  };
}
