/**
 * The llmstxt.org index for the site: what Vibe is, and where an agent should read first.
 *
 * Pure: the build (scripts/build-markdown.mjs) gathers the entries and this only writes them.
 * Each entry is `- [title](url): note — Markdown: <twin>`, and a section is written only when
 * it has entries, so a missing API leaves out the dynamic sections and nothing else.
 */

/** A link text must not close its own brackets early. */
const label = (text) => String(text).replace(/\[/g, '(').replace(/\]/g, ')');

function entry({ title, url, md, note }) {
  const tail = note ? `: ${note}` : '';
  const twin = md ? ` — Markdown: ${md}` : '';
  return `- [${label(title)}](${url})${tail}${twin}`;
}

function section(heading, entries) {
  if (!entries.length) return null;
  return `## ${heading}\n\n${entries.map(entry).join('\n')}`;
}

/**
 * @param {object} data
 * @param {string} data.title          H1, "Vibe".
 * @param {string} data.description    The home description, the blockquote.
 * @param {string} data.summary        One paragraph on what the site is.
 * @param {string} data.updated        ISO date of the newest content change.
 * @param {Array}  data.pages          Static pages: { title, url, md, note }.
 * @param {Array}  data.cities         City hubs and the index: { title, url, md, note }.
 * @param {Array}  data.complexes      Complex pages: { title, url, md, note }.
 */
export function renderLlmsTxt({ title, description, summary, updated, pages, cities, complexes }) {
  const intro = [
    `# ${title}`,
    '',
    `> ${description}`,
    '',
    summary,
    '',
    'Toda página de este sitio responde en Markdown si la pedís con la cabecera ' +
      '`Accept: text/markdown`, y en HTML si no. Los archivos `.md` listados abajo son ' +
      'ese mismo contenido, accesible directamente.',
    '',
    `Última actualización: ${updated}`,
  ];

  const sections = [
    section('Páginas', pages),
    section('Canchas', cities),
    section('Complejos', complexes),
  ].filter(Boolean);

  return `${intro.join('\n')}\n\n${sections.join('\n\n')}\n`;
}
