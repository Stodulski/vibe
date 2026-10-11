/**
 * Copy helpers shared by the storefront's head builders (hub.ts, complex-meta.ts,
 * hub-index.ts). Pure: no imports, so node:test runs them directly.
 */

/** "a", "a y b", "a, b y c". */
export function joinSpanish(items: readonly string[]): string {
  if (items.length <= 1) return items.join('');
  return `${items.slice(0, -1).join(', ')} y ${items[items.length - 1]}`;
}

/**
 * Trims a name or a label and collapses its inner runs of whitespace to one space, so a
 * stored "Afa " or "La  Zona" reaches the copy as "Afa" and "La Zona".
 */
export function normalizeText(text: string): string {
  return text.replace(/\s+/g, ' ').trim();
}

/**
 * Cuts text to at most `limit` characters at a word boundary, ending with "...". Text
 * within the limit is returned as it is. Counts characters, not bytes, so a cut never
 * splits a letter.
 */
export function cutAtWord(text: string, limit: number): string {
  const characters = Array.from(text);
  if (characters.length <= limit) return text;
  const head = characters.slice(0, limit - 3).join('');
  const boundary = head.lastIndexOf(' ');
  return `${boundary > 0 ? head.slice(0, boundary) : head}...`;
}

/**
 * The two JavaScript line separators. JSON leaves them unescaped, but they end a line in
 * a script element, so they are escaped too. Built from char codes so the source stays
 * ASCII.
 */
const LINE_SEPARATORS = new RegExp(`[${String.fromCharCode(0x2028, 0x2029)}]`, 'g');

/**
 * JSON for a `<script type="application/ld+json">` body. The payload carries venue
 * names and addresses typed by owners, so `<`, `>`, `&` and the two line separators are
 * written as escapes: a name containing `</script>` cannot close the element.
 */
export function jsonLdScript(data: unknown): string {
  return JSON.stringify(data)
    .replace(/</g, '\\u003c')
    .replace(/>/g, '\\u003e')
    .replace(/&/g, '\\u0026')
    .replace(LINE_SEPARATORS, (char) => `\\u${char.charCodeAt(0).toString(16)}`);
}
