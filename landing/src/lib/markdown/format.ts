/**
 * Small Markdown writers shared by the storefront twins. They emit plain CommonMark with
 * absolute links, so a file read on its own, away from the site, still points somewhere.
 */

export const SITE = 'https://vibe.com.ar';

/** Escapes what Markdown or HTML would read as syntax in running text, so a name stays a name. */
export function escapeInline(text: string): string {
  return text.replace(/[\\`*_[\]<|]/g, (ch) => `\\${ch}`);
}

/** An absolute URL for a site path, so the document reads correctly detached from the site. */
export function absolute(path: string, origin: string = SITE): string {
  return /^https?:\/\//.test(path) ? path : `${origin}${path}`;
}

/** A link with escaped text and an absolute target. */
export function link(text: string, url: string): string {
  return `[${escapeInline(text)}](${url})`;
}

/** A heading at the given level. Headings never skip a level, so callers keep them in order. */
export function heading(level: 1 | 2 | 3, text: string): string {
  return `${'#'.repeat(level)} ${escapeInline(text)}`;
}

/** A bullet list, one item per line. Empty input yields nothing, so callers can drop the block. */
export function bullets(items: readonly string[]): string {
  return items.map((item) => `- ${item}`).join('\n');
}

/** Question and answer as an H3 with its answer under it. */
export function faqBlock(question: string, answer: string): string {
  return `${heading(3, question)}\n\n${escapeInline(answer)}`;
}

/** YAML front matter, quoting the values as JSON does, which YAML accepts for double quotes. */
export function frontMatter(fields: {
  title: string;
  description?: string | undefined;
  canonical: string;
  lastModified?: string | undefined;
}): string {
  const lines = [
    '---',
    `title: ${JSON.stringify(fields.title)}`,
    fields.description ? `description: ${JSON.stringify(fields.description)}` : null,
    `canonical: ${fields.canonical}`,
    fields.lastModified ? `lastModified: ${fields.lastModified}` : null,
    '---',
  ];
  return lines.filter((line): line is string => line !== null).join('\n');
}

/** Joins the blocks with blank lines, drops the empty ones, and ends the file with one newline. */
export function document(front: string, blocks: readonly (string | null | undefined | false)[]): string {
  const body = blocks.filter((block): block is string => typeof block === 'string' && block.trim() !== '');
  return `${front}\n\n${body.join('\n\n')}\n`;
}
