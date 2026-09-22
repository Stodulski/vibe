/**
 * An empty note textarea sends no field at all, not `""` — same
 * "falsy-to-undefined" rule as `cleanBookingPayload`'s `blankToUndefined`,
 * kept as a plain `if` (not `||`/`??`/a ternary): `??` only catches
 * `null`/`undefined`, not `''`, which would change the meaning, and ESLint's
 * `prefer-nullish-coalescing` flags the ternary shape regardless.
 */
export function blankNoteToUndefined(note: string | undefined): string | undefined {
  if (!note) return undefined;
  return note;
}
