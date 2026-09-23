/**
 * An empty text field (note, category) sends no field at all, not `""` —
 * same "falsy-to-undefined" rule as `cleanBookingPayload`'s own
 * `blankToUndefined`, kept as a plain `if` (not `||`/`??`/a ternary): `??`
 * only catches `null`/`undefined`, not `''`, which would change the meaning,
 * and ESLint's `prefer-nullish-coalescing` flags the ternary shape regardless.
 *
 * Moved here from `features/products/lib/blankToUndefined.ts`
 * (pos-products-screen T5a): `features/cash` had duplicated this verbatim as
 * `blankNoteToUndefined` on the "features never import from one another"
 * reasoning — the repo's rule for that case is to move the shared piece to
 * `shared/` instead, same move as `shared/lib/paymentMethods.ts`.
 * `cleanBookingPayload`'s own copy (`features/bookings`) is out of scope
 * here: it is generic over string literal unions, a different shape.
 */
export function blankToUndefined(value: string | undefined): string | undefined {
  if (!value) return undefined;
  return value;
}
