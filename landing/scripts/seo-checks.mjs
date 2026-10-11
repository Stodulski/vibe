/**
 * Checks the SEO gate (scripts/seo-gate.mjs) and the smoke test (tests/smoke.mjs) share:
 *
 * - placeholder data that must never reach the storefront: obvious junk words in a title or
 *   heading, and phone numbers made of one repeated digit run (+541111111111);
 * - JSON-LD dateModified values that disagree with the page's sitemap lastmod, which means
 *   src/data/page-dates.json was not regenerated and committed with the change.
 *
 * Pure: no network and no files, so node:test can run it.
 */

/** Words that only ever appear in test data. Matched as whole words, so "testeo" is not one. */
export const JUNK_WORDS = ['asdasd', 'asdf', 'qwerty', 'test', 'prueba'];

const JUNK = new RegExp(`(^|[^\\p{L}\\p{N}])(${JUNK_WORDS.join('|')})(?=$|[^\\p{L}\\p{N}])`, 'iu');

/** The placeholder word in the text, lowercased, or null. */
export function junkWordIn(text) {
  const match = String(text).match(JUNK);
  return match ? match[2].toLowerCase() : null;
}

/** A phone with 8 or more identical digits in a row, like +541111111111, is placeholder data. */
export function placeholderPhone(phone) {
  const digits = String(phone).replace(/\D/g, '');
  if (digits === '') return false;
  return /(\d)\1{7,}/.test(digits);
}

const tagText = (html, re) => (html.match(re)?.[1] ?? '').replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim();

/**
 * The placeholder problems of one rendered page: junk words in its <title> and <h1>, and
 * placeholder numbers in its WhatsApp or tel: links. One message per word or number.
 */
export function placeholderProblemsInHtml(html) {
  const problems = [];
  const title = tagText(html, /<title[^>]*>([\s\S]*?)<\/title>/i);
  const h1 = tagText(html, /<h1[^>]*>([\s\S]*?)<\/h1>/i);
  for (const [where, text] of [['el título', title], ['el H1', h1]]) {
    const word = junkWordIn(text);
    if (word) problems.push(`placeholder "${word}" en ${where}: ${text}`);
  }
  for (const match of String(html).matchAll(/(?:wa\.me\/|tel:)\+?([\d\s()-]+)/g)) {
    if (placeholderPhone(match[1])) problems.push(`teléfono placeholder ${match[1].trim()}`);
  }
  return problems;
}

/**
 * JSON-LD dates that disagree with the sitemap, for the routes whose dateModified comes from
 * page-dates.json. Any other route is not compared: a guide's date is its own.
 */
export function dateDriftProblems({ jsonLd, sitemap, routes }) {
  const problems = [];
  for (const route of routes) {
    const shown = jsonLd.get(route);
    const listed = sitemap.get(route);
    if (shown === undefined || listed === undefined || shown === listed) continue;
    problems.push(
      `${route}: la fecha del JSON-LD es ${shown} y el sitemap dice ${listed}. ` +
        'Corré `pnpm build` y commiteá src/data/page-dates.json.',
    );
  }
  return problems;
}
