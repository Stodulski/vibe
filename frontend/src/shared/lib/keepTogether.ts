/** Swaps regular spaces for non-breaking ones so a name never wraps across lines. */
export function keepTogether(text: string): string {
  return text.replace(/ /g, ' ');
}
