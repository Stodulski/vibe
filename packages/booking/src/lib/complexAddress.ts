/**
 * The address on one short line: street and city, from the parts the payload has. The
 * province is left out: the city already places the club, and the line fits a phone.
 * A city that repeats the street appears once, and a blank part is skipped.
 */
export function shortAddress(parts: { address: string; city: string }): string {
  return [parts.address, parts.city]
    .map((part) => part.trim())
    .filter((part, index, all) => part !== '' && all.indexOf(part) === index)
    .join(', ');
}
