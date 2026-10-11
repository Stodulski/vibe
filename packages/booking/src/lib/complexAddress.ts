/**
 * The address on one line: street, city, province, from the parts the payload has.
 * A part that repeats an earlier one (a city that is also its province) appears once,
 * and a blank part is skipped.
 */
export function fullAddress(parts: { address: string; city: string; province: string }): string {
  return [parts.address, parts.city, parts.province]
    .map((part) => part.trim())
    .filter((part, index, all) => part !== '' && all.indexOf(part) === index)
    .join(', ');
}
