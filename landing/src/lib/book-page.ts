/**
 * Where /c/<slug>/book sends a visitor. MercadoPago's failure back_url lands here with
 * `?error=payment_failed` and gets the failure screen. Any other visit is sent on to the
 * complex page, keeping the query so the slot picker still has its answers.
 */
export type BookPageAction = { kind: 'payment_failed' } | { kind: 'redirect'; location: string };

export function bookPageAction(slug: string, search: URLSearchParams): BookPageAction {
  if (search.get('error') === 'payment_failed') return { kind: 'payment_failed' };
  const query = search.toString();
  return { kind: 'redirect', location: `/c/${encodeURIComponent(slug)}${query ? `?${query}` : ''}` };
}
