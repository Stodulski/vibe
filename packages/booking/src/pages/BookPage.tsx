import { useEffect } from 'react';
import { publicComplexPath } from '../lib/publicPaths';
import { PaymentFailedScreen } from './book-page/PaymentFailedScreen';
import { navigateTo, useQueryParams } from '../lib/navigation';

/**
 * Where MercadoPago sends someone whose payment was rejected.
 *
 * Nothing in this app navigates here, which is why this file once carried a
 * comment saying it was no longer needed — and it nearly got deleted on the
 * strength of it. The caller is on the server: `booklink.Failure` builds the
 * checkout preference's failure back_url as `/c/{slug}/book?error=payment_failed`
 * (backend, `internal/booklink/booklink.go`), so every rejected card
 * lands on this route. Deleting it turns the worst moment of the flow into a
 * 404.
 *
 * It used to forward the `?error=` on to the complex page, which raised a
 * toast over the availability grid. The failure is a destination, not a
 * notification, so it is answered here. Anything else reaching this URL —
 * an old bookmark, a stray link — still belongs on the complex page.
 *
 * Before removing this route, change that back_url first.
 */
export default function BookPage({ slug }: { slug: string }) {
  const [searchParams] = useQueryParams();
  const paymentFailed = searchParams.get('error') === 'payment_failed';

  useEffect(() => {
    if (!paymentFailed) {
      navigateTo(publicComplexPath(slug), { replace: true });
    }
  }, [paymentFailed, slug]);

  if (paymentFailed) {
    return <PaymentFailedScreen slug={slug} />;
  }

  return null;
}
