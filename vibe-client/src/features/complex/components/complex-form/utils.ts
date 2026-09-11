// Plain if/return (not a ternary/`||`/`??`) so an empty string intentionally
// also falls through to `undefined` — `??` only catches `null`/`undefined`
// and would keep `''` — same precedent as
// `create-booking-modal/cleanBookingPayload.ts`.
export function blankToUndefined(value: string | undefined): string | undefined {
  if (!value) return undefined;
  return value;
}

export function slugify(text: string): string {
  return text
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-|-$/g, '');
}
