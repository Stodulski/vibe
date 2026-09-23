import { describe, expect, it } from 'vitest';
import { HTTPError } from 'ky';
import type { NormalizedOptions } from 'ky';
import { ES_AR } from '@/shared/i18n/es_AR';
import { getFieldErrors, translateServerError } from '@/shared/lib/serverErrors';

/** ky consumes the body before throwing, so the payload lives on `.data`. */
function httpError(status: number, body: unknown): HTTPError {
  const response = new Response(JSON.stringify(body), { status });
  const request = new Request('https://api.vibe.com.ar/api/v1/test', { method: 'POST' });
  const error = new HTTPError(response, request, {} as NormalizedOptions);
  error.data = body;
  return error;
}

describe('translateServerError', () => {
  it('turns a known code into its Spanish text', () => {
    expect(translateServerError('slug_taken')).toBe(ES_AR.validation.server.slugTaken);
    expect(translateServerError('court_name_taken')).toBe(ES_AR.validation.server.courtNameTaken);
  });

  // The backend still returns prose for messages that have not been converted
  // to codes. Showing an imperfect one beats replacing it with a generic that
  // says less.
  it('passes an unknown value through unchanged', () => {
    expect(translateServerError('cannot delete court while it has active bookings')).toBe(
      'cannot delete court while it has active bookings',
    );
  });

  // The products API's own 409 prose (pos-cashbox T5a), matched by exact
  // text rather than a code — see `serverErrors.ts`'s own comment.
  it('turns "cannot stop tracking stock..." into the stock-not-zero copy', () => {
    expect(translateServerError('cannot stop tracking stock while stock_on_hand is not zero')).toBe(
      ES_AR.products.stockNotZero,
    );
  });

  it('turns "this product is not active" into the product-inactive copy', () => {
    expect(translateServerError('this product is not active')).toBe(ES_AR.products.productInactive);
  });

  it('turns "this product does not track stock" into the not-tracking-stock copy', () => {
    expect(translateServerError('this product does not track stock')).toBe(ES_AR.products.productNotTrackingStock);
  });

  // Shared prose: both `products` (restock) and `sales` (POS) answer a
  // closed till with this exact text, so the global mapping stays neutral —
  // never the restock-specific "Para reponer necesitás..." copy, which only
  // `RestockDialog` shows, driven by the till state it reads itself.
  it('turns "no cash session is open" into the neutral cash-closed copy, not the restock-specific one', () => {
    expect(translateServerError('no cash session is open')).toBe(ES_AR.validation.server.cashClosed);
    expect(translateServerError('no cash session is open')).not.toBe(ES_AR.products.restockNeedsOpenTill);
  });
});

describe('getFieldErrors', () => {
  it('reads the field map a failed validation returns', () => {
    const fields = getFieldErrors(
      httpError(422, { title: 'Validation failed', errors: [{ field: 'slug', message: 'slug_taken' }] }),
    );

    expect(fields).toEqual({ slug: ES_AR.validation.server.slugTaken });
  });

  it('translates every field it finds', () => {
    const fields = getFieldErrors(
      httpError(422, {
        title: 'Validation failed',
        errors: [
          { field: 'deposit_amount', message: 'deposit_exceeds_price' },
          { field: 'slug', message: 'slug_taken' },
        ],
      }),
    );

    expect(fields.deposit_amount).toBe(ES_AR.validation.server.depositExceedsPrice);
    expect(fields.slug).toBe(ES_AR.validation.server.slugTaken);
  });

  it.each([
    ['a failure with no fields at all', httpError(500, { title: 'Server error' })],
    ['a non-HTTP failure', new Error('boom')],
    ['no error at all', undefined],
  ])('returns nothing for %s', (_label, error) => {
    expect(getFieldErrors(error)).toEqual({});
  });
});
