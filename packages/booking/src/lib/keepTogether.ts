/**
 * Joins the words of a name with non-breaking spaces so it never wraps onto a
 * second line. The owner's rule: a court's name always stays together.
 */
export function keepTogether(text: string): string {
  return text.replace(/ /g, ' ');
}
