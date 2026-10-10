import { PaymentFailedEntry } from '@vibe/booking/payment-failed';
import { getPublicBookingConfig } from '../publicBookingRuntime';
import { useRouteSlug } from './useRouteSlug';

// Not a page anyone navigates to: MercadoPago's failure back_url. See
// PaymentFailedEntry for why the route must not be removed.
export function BookPageRoute() {
  const slug = useRouteSlug();
  return <PaymentFailedEntry slug={slug} config={getPublicBookingConfig()} toaster={false} />;
}
