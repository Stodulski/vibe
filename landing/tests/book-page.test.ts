/**
 * The routing of /c/<slug>/book: the MercadoPago failure screen, or a redirect to the
 * complex page that keeps the query. Run with `pnpm test:unit`.
 */
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { bookPageAction } from '../src/lib/book-page.ts';

test('a failed payment gets the failure screen', () => {
  assert.deepEqual(bookPageAction('club-sur', new URLSearchParams('error=payment_failed')), {
    kind: 'payment_failed',
  });
});

test('any other error value is not a failed payment', () => {
  assert.deepEqual(bookPageAction('club-sur', new URLSearchParams('error=other')), {
    kind: 'redirect',
    location: '/c/club-sur?error=other',
  });
});

test('a visit without a query goes to the complex page', () => {
  assert.deepEqual(bookPageAction('club-sur', new URLSearchParams()), {
    kind: 'redirect',
    location: '/c/club-sur',
  });
});

test('the redirect keeps the slot picker answers', () => {
  assert.deepEqual(bookPageAction('club-sur', new URLSearchParams('date=2026-10-20&sport=padel')), {
    kind: 'redirect',
    location: '/c/club-sur?date=2026-10-20&sport=padel',
  });
});

test('the slug is encoded in the redirect', () => {
  assert.deepEqual(bookPageAction('club sur', new URLSearchParams()), {
    kind: 'redirect',
    location: '/c/club%20sur',
  });
});
