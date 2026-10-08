import { Navigate, useLocation } from 'react-router-dom';

/**
 * A complex storefront's pre-`/c/` address: `/<slug>`, or one of its booking
 * steps `/<slug>/book[/confirm|/success|/cancel]`. Emails, WhatsApp messages
 * and MercadoPago back_urls already carry these, so they keep working.
 *
 * Every legacy address is the `/c/` address without the prefix, so the
 * redirect just puts the prefix back. The query string and hash travel with it
 * (the success page's `token`, the failure page's `error`). The route is
 * replaced rather than pushed, so Back does not return to the old address.
 *
 * Only the explicit legacy paths in `publicRoutes.tsx` reach this component.
 * react-router ranks static segments above dynamic ones, so platform routes
 * (`/login`, `/cash/sell`, ...) are never handed here.
 */
export function LegacyComplexRedirect() {
  const { pathname, search, hash } = useLocation();

  return <Navigate replace to={{ pathname: `/c${pathname}`, search, hash }} />;
}
