/**
 * Post-build pass that produces everything an AI agent reads instead of the page:
 *
 *   dist/<route>.md   a Markdown twin of each page, served at the page's own URL
 *                     when the request carries `Accept: text/markdown`
 *                     (see middleware.ts — vercel.json rewrites cannot do this,
 *                     they are evaluated after the filesystem lookup)
 *   dist/sitemap.xml  regenerated, each page carrying the date its own output
 *                     last changed (see src/data/page-dates.json)
 *   src/data/llms-static.json  the static part of llms.txt, which /llms.txt renders
 *                     per request (src/pages/llms.txt.ts). Committed, not written to dist.
 *
 * All of it is derived from the built HTML rather than written by hand: the copy
 * on this site changes often, and a second hand-maintained copy would go stale
 * the first time nobody remembered it existed.
 */
import { readFile, writeFile, readdir } from 'node:fs/promises';
import { readFileSync, writeFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { execSync } from 'node:child_process';
import { join, relative } from 'node:path';
import TurndownService from 'turndown';
import { esIndexable } from './seo-checks.mjs';

const DIST = new URL('../dist/', import.meta.url).pathname;
const SITE = 'https://vibe.com.ar';

/* Reading order: the home, then the guides hub and its guides, then the rest.
   It is what an agent should read first, not alphabetical. */
function ordenDeLectura(route) {
  if (route === '/') return [0, route];
  if (route === '/guias') return [1, route];
  if (route.startsWith('/guias/')) return [2, route];
  return [3, route];
}

/* Everything here is layout, not content: carousel clones repeat slides that
   already appear once, the mobile tab strip repeats the comparison, and the
   arrows and dots are controls with nothing to say to a reader. */
const DROP_CLASSES = [
  'ft-block--clone',
  'ft-arrow',
  'ft-dots',
  'comparison-tabs',
  'skip-link',
  /* Eyebrow labels ("Precio", "Funcionalidades") only repeat the heading that
     follows them, and the numbered node is decoration for an ordered list. */
  'section-label',
  'how-step-node',
  'how-step-spacer',
];

const turndown = new TurndownService({
  headingStyle: 'atx',
  bulletListMarker: '-',
  codeBlockStyle: 'fenced',
  emDelimiter: '*',
});

turndown.remove(['script', 'style', 'noscript', 'svg', 'head']);

turndown.addRule('drop-decorative', {
  filter(node) {
    if (node.nodeType !== 1) return false;
    if (node.getAttribute('aria-hidden') === 'true') return true;
    const cls = node.getAttribute('class') || '';
    return DROP_CLASSES.some(c => cls.split(/\s+/).includes(c));
  },
  replacement: () => '',
});

/* The nav is a logo plus anchors to the sections printed right below it. */
turndown.addRule('drop-nav', {
  filter: node => node.nodeName === 'NAV',
  replacement: () => '',
});

const txt = n => (n?.textContent || '').replace(/\s+/g, ' ').trim();
const hasClass = (n, c) => n.nodeType === 1 && (n.getAttribute('class') || '').split(/\s+/).includes(c);
/* "<5min" would open an HTML tag once it is Markdown. */
const esc = s => s.replace(/</g, '\\<');
const cell = s => esc(s).replace(/\|/g, '\\|');

/* A heading carrying a <br> was split in two, leaving its second half adrift as
   a separate paragraph. textContent alone is not enough either: it drops the <br>
   without leaving anything behind, welding the two halves into one word. */
function headingText(node) {
  let out = '';
  (function walk(n) {
    if (n.nodeType === 3) out += n.textContent;
    else if (n.nodeName === 'BR') out += ' ';
    else if (n.childNodes) Array.from(n.childNodes).forEach(walk);
  })(node);
  return out.replace(/\s+/g, ' ').trim();
}

turndown.addRule('one-line-heading', {
  filter: n => /^H[1-6]$/.test(n.nodeName),
  replacement: (_, node) => `\n\n${'#'.repeat(+node.nodeName[1])} ${esc(headingText(node))}\n\n`,
});

/* A figure means nothing without its label, and they were landing three lines
   apart: "24/7", blank, "Reservas sin intervención". */
turndown.addRule('stat', {
  filter: n => hasClass(n, 'stat'),
  replacement(_, node) {
    const v = txt(node.querySelector('.stat-value'));
    const l = txt(node.querySelector('.stat-label'));
    return v && l ? `\n- ${esc(v)}: ${esc(l)}` : '';
  },
});

turndown.addRule('fee-row', {
  filter: n => n.nodeName === 'LI' && n.querySelector && n.querySelector('.mp-fees-term'),
  replacement: (_, node) =>
    `\n- ${esc(txt(node.querySelector('.mp-fees-term')))}: ${esc(txt(node.querySelector('.mp-fees-rate')))}`,
});

turndown.addRule('trust-badge', {
  filter: n => hasClass(n, 'trust-badge'),
  replacement: (_, node) => `\n- ${esc(txt(node))}`,
});

/* Turndown has no table support of its own, and this one carries the answer to
   "which should I pick": printed as loose paragraphs it says nothing. The per-cell
   mark is the accessible stand-in for the page's green/grey/red colouring. */
turndown.addRule('comparison', {
  filter: n => hasClass(n, 'comparison-table'),
  replacement(_, node) {
    const marcas = { good: '\u2705', bad: '\u274c' };
    const encabezados = Array.from(node.querySelectorAll('thead th')).map(th => {
      const img = th.querySelector('img');
      return (img && img.getAttribute('alt')) || txt(th);
    });
    if (!encabezados.length) return '';
    const filas = Array.from(node.querySelectorAll('tbody tr')).map(tr =>
      Array.from(tr.querySelectorAll('td')).map(td => {
        const tipo = ['good', 'bad'].find(k => hasClass(td, `comparison-${k}`));
        return `${marcas[tipo] || '\u2796'} ${cell(txt(td))}`;
      }));
    const caption = txt(node.querySelector('caption'));
    return `\n\n${caption ? caption + '\n\n' : ''}| ${encabezados.map(cell).join(' | ')} |\n`
         + `| ${encabezados.map(() => '---').join(' | ')} |\n`
         + filas.map(f => `| ${f.join(' | ')} |`).join('\n') + '\n\n';
  },
});

/* Turndown no sabe de tablas, y la de una guia es justamente el dato que un
   agente vendria a buscar: sin esta regla se desarma en una columna de lineas
   sueltas ("Provincia", "Al instante", "6,60%"...) donde ya no se sabe que
   porcentaje corresponde a que provincia. */
turndown.addRule('guia-tabla', {
  filter: n => hasClass(n, 'guia-tabla'),
  replacement(_, node) {
    const encabezados = Array.from(node.querySelectorAll('thead th')).map(txt);
    if (!encabezados.length) return '';
    const filas = Array.from(node.querySelectorAll('tbody tr'))
      .map(tr => Array.from(tr.querySelectorAll('th, td')).map(c => cell(txt(c))));
    return `\n\n| ${encabezados.map(cell).join(' | ')} |\n`
         + `| ${encabezados.map(() => '---').join(' | ')} |\n`
         + filas.map(f => `| ${f.join(' | ')} |`).join('\n') + '\n\n';
  },
});

/* La linea de fecha y fuente separa sus partes con un `·` aria-hidden. No
   alcanza con procesar el resultado ya convertido: para cuando llega, la regla
   de decoracion ya borro el separador y las partes quedaron pegadas
   ("Actualizado el 2026-08-24Fuente:"). Hay que recorrer los hijos y volver a
   unirlos con un separador propio. */
turndown.addRule('guia-meta', {
  filter: n => hasClass(n, 'guia-meta'),
  replacement(_, node) {
    const partes = Array.from(node.children)
      .filter(hijo => hijo.getAttribute('aria-hidden') !== 'true')
      .map(hijo => {
        const enlace = hijo.querySelector('a[href]');
        if (!enlace) return esc(txt(hijo));
        /* "Fuente: MercadoPago, costos de Checkout" -> el prefijo suelto mas el
           enlace, para no perder ni la etiqueta ni la URL. */
        const prefijo = txt(hijo).replace(txt(enlace), '').trim();
        return `${prefijo ? esc(prefijo) + ' ' : ''}[${esc(txt(enlace))}](${enlace.getAttribute('href')})`;
      })
      .filter(Boolean);
    return partes.length ? `\n\n${partes.join(' · ')}\n\n` : '';
  },
});

/* Question and answer were two loose paragraphs with nothing tying them. */
turndown.addRule('faq', {
  filter: n => n.nodeName === 'DETAILS' && hasClass(n, 'faq-item'),
  replacement: (_, node) =>
    `\n\n### ${esc(txt(node.querySelector('summary')))}\n\n${esc(txt(node.querySelector('p')))}\n`,
});

/* Icon-only links came out as "[](https://...)". The accessible name is the
   label a reader needs. */
turndown.addRule('icon-link', {
  filter: n => n.nodeName === 'A' && !txt(n) && n.getAttribute('aria-label'),
  replacement: (_, node) => `[${esc(node.getAttribute('aria-label'))}](${node.getAttribute('href')})`,
});

const attr = (html, re) => (html.match(re)?.[1] ?? '').trim();

const ISO = /^\d{4}-\d{2}-\d{2}$/;

const FALLBACK = new Date().toISOString().slice(0, 10);

function gitDate() {
  try {
    const out = execSync('git log -1 --format=%cs', {
      encoding: 'utf8',
      stdio: ['ignore', 'pipe', 'ignore'],
    }).trim();
    return ISO.test(out) ? out : null;
  } catch {
    return null;
  }
}

const HOY = gitDate() || FALLBACK;

/* Per-page dates cannot come from `git log -- <file>`: Vercel builds from a
   shallow clone, where every file reports the head commit whether it changed or
   not. They are derived from the page's own output instead and kept in this
   file, so a page only gets a new date when what it renders actually differs.
   A lastmod that moves while the page did not is one a search engine learns to
   distrust. */
const DATES_FILE = new URL('../src/data/page-dates.json', import.meta.url).pathname;

/* Dates are stripped before hashing, or writing a new date would itself change
   the page and bump the date again on the next build. */
function huella(html) {
  const sinFechas = html
    .replace(/\d{4}-\d{2}-\d{2}/g, '')
    .replace(/<script type="application\/ld\+json"[^>]*>[\s\S]*?<\/script>/g, '');
  return createHash('sha256').update(sinFechas).digest('hex').slice(0, 16);
}

function leerDates() {
  try { return JSON.parse(readFileSync(DATES_FILE, 'utf8')); } catch { return {}; }
}

function leerTexto(file) {
  try { return readFileSync(file, 'utf8'); } catch { return ''; }
}

/* What every real client downloads, which is what the sitemap should describe.
   The <img src> is only a fallback: browsers take the WebP from <source>, and
   Googlebot renders mobile-first, so it never sees the PNG at all. Both the
   mobile and desktop sources are listed because both are genuinely served,
   depending on the viewport. SVG is left out: image search does not index it. */
function rasterImages(html) {
  const fromSources = [...html.matchAll(/<source[^>]*\bsrcset="([^"]+)"/gi)]
    .map(m => m[1].split(',')[0].trim().split(/\s+/)[0]);
  const fromImgs = [...html.matchAll(/<img[^>]*\bsrc="([^"]+)"/gi)].map(m => m[1]);

  /* /emoji/ holds the hero's decorative sport emojis: not content anyone
     searches for, so they stay out of the image sitemap. */
  const all = [...fromSources, ...fromImgs].filter(u => /\.(png|jpe?g|webp)$/i.test(u) && !u.startsWith('/emoji/'));
  /* Drop a PNG when the same picture is also offered as WebP. */
  const stems = new Set(all.filter(u => /\.webp$/i.test(u)).map(u => u.replace(/\.webp$/i, '')));
  return [...new Set(all.filter(u => /\.webp$/i.test(u) || !stems.has(u.replace(/\.(png|jpe?g)$/i, ''))))];
}

const xml = s => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

function metaDescription(html) {
  /* Astro emits attributes alphabetically, so `content` lands before `name`. */
  return attr(html, /<meta[^>]*\bcontent="([^"]*)"[^>]*\bname="description"[^>]*>/i);
}

function toMarkdown(html, route, lastModified) {
  const title = attr(html, /<title[^>]*>([\s\S]*?)<\/title>/i);
  const description = metaDescription(html);
  const body = attr(html, /<body[^>]*>([\s\S]*?)<\/body>/i) || html;

  const front = [
    '---',
    `title: ${JSON.stringify(title)}`,
    description && `description: ${JSON.stringify(description)}`,
    `canonical: ${SITE}${route}`,
    lastModified && `lastModified: ${lastModified}`,
    '---',
  ].filter(Boolean).join('\n') + '\n\n';

  const md = turndown.turndown(body)
    /* Two buttons side by side became "[uno](...)[dos](...)", which reads as one
       broken link. */
    .replace(/\)\[([^\]]+)\]\(/g, ')\n\n[$1](')
    /* The file is read on its own, detached from the site, so a bare "/privacidad"
       or "#precio" points at nothing. */
    .replace(/\]\((\/[^)\s]*)\)/g, `](${SITE}$1)`)
    .replace(/\]\((#[^)\s]*)\)/g, `](${SITE}${route === '/' ? '/' : route}$1)`)
    /* Turndown pads its own bullets to "-   " while the rules above emit "- ";
       one document should not use two. There are no nested lists here, so the
       padding buys nothing. */
    .replace(/^-   (?=\S)/gm, '- ')
    .replace(/\n{3,}/g, '\n\n')
    .trim();
  return { title, description, markdown: `${front}${md}\n` };
}

async function htmlFiles(dir) {
  const out = [];
  for (const entry of await readdir(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) out.push(...await htmlFiles(full));
    else if (entry.name.endsWith('.html')) out.push(full);
  }
  return out;
}

/* dist/index.html -> "/" and dist/privacidad/index.html -> "/privacidad", which
   is how the site is actually addressed (vercel.json sets cleanUrls). */
function routeOf(file) {
  const rel = relative(DIST, file).replace(/\\/g, '/');
  if (rel === 'index.html') return '/';
  return '/' + rel.replace(/\/index\.html$/, '').replace(/\.html$/, '');
}

/* Flat filenames so each one is a single rewrite target: "/" -> index.md. */
function fileNameOf(route) {
  return (route === '/' ? 'index' : route.slice(1).replace(/\//g, '-')) + '.md';
}

/* The page states plainly what Vibe is, in one paragraph, for exactly this
   purpose. Reusing it beats writing a second summary that can contradict it. */
function summaryFrom(markdown, fallback) {
  const m = markdown.match(/##\s*¿Qué es Vibe\?\s*\n+([^\n]+)/);
  return (m?.[1] ?? fallback).trim();
}

const DATES = leerDates();

/* Guides and tutorials state their own dates, from the same table their JSON-LD reads
   (src/data/guias-fechas.json, keyed by slug and by "tutorial:<slug>"). The tutorials
   index states the newest of its tutorials. Those routes are not tracked in
   page-dates.json: a second copy of their date would drift from the one in the page. */
const FECHAS_DATOS = (() => {
  try { return JSON.parse(readFileSync(new URL('../src/data/guias-fechas.json', import.meta.url), 'utf8')); } catch { return {}; }
})();

function fechaDeDatos(route) {
  if (route.startsWith('/guias/')) return FECHAS_DATOS[route.slice('/guias/'.length)]?.date;
  if (route.startsWith('/tutoriales/')) return FECHAS_DATOS[`tutorial:${route.slice('/tutoriales/'.length)}`]?.date;
  if (route === '/tutoriales') {
    return Object.entries(FECHAS_DATOS)
      .filter(([clave]) => clave.startsWith('tutorial:'))
      .map(([, registro]) => registro.date)
      .sort()
      .at(-1);
  }
  return undefined;
}

/* The date a page states: its data's date when it has one, else its own entry. */
const fechaDe = route => fechaDeDatos(route) ?? DATES[route]?.date;

const pages = new Map();
for (const file of await htmlFiles(DIST)) {
  const route = routeOf(file);
  const html = await readFile(file, 'utf8');
  pages.set(route, {
    html,
    title: attr(html, /<title[^>]*>([\s\S]*?)<\/title>/i),
    description: metaDescription(html),
  });
}

/* Ahora que cada pagina esta leida se sabe cual declara noindex. */
const INDEXED = [...pages.entries()]
  .filter(([, p]) => esIndexable(p.html))
  .map(([route]) => route)
  .sort((a, b) => {
    const [pa, ra] = ordenDeLectura(a);
    const [pb, rb] = ordenDeLectura(b);
    return pa - pb || ra.localeCompare(rb, 'es');
  });

console.log(`  indexables  ${INDEXED.join(', ')}`);

/* A page keeps its stored date while its output is unchanged, and takes today's
   only when it actually differs. New pages start at today. The dates come first,
   because each Markdown twin states its page's date. */
let datesCambiaron = false;
for (const route of INDEXED) {
  if (fechaDeDatos(route) !== undefined) continue;
  const h = huella(pages.get(route).html);
  const previo = DATES[route];
  if (!previo || previo.hash !== h) {
    DATES[route] = { hash: h, date: HOY };
    datesCambiaron = true;
    console.log(`  fecha  ${route} -> ${HOY} (cambio el contenido)`);
  }
}
if (datesCambiaron) {
  writeFileSync(DATES_FILE, JSON.stringify(DATES, null, 2) + '\n', 'utf8');
  console.log('  src/data/page-dates.json actualizado — commitealo con el cambio');
}

for (const [route, page] of pages) {
  const { markdown } = toMarkdown(page.html, route, fechaDe(route));
  page.markdown = markdown;
  await writeFile(join(DIST, fileNameOf(route)), markdown, 'utf8');
  console.log(`  markdown  ${route} -> ${fileNameOf(route)}`);
}

/* The static part of llms.txt: what the site is and the pages it lists. /llms.txt is rendered
   on every request by src/pages/llms.txt.ts, and a function cannot read dist/, so this part
   lives in a committed file the function imports, the same way page-dates.json works. The
   city and complex sections are read from the API at request time, not here. */
const LLMS_STATIC_FILE = new URL('../src/data/llms-static.json', import.meta.url).pathname;

const home = pages.get('/');
const llmsStatic = {
  title: 'Vibe',
  description: home?.description ?? '',
  summary: summaryFrom(home?.markdown ?? '', home?.description ?? ''),
  updated: INDEXED.map(fechaDe).filter(Boolean).sort().at(-1) ?? HOY,
  pages: INDEXED.filter(r => pages.has(r)).map(r => ({
    title: pages.get(r).title,
    url: `${SITE}${r}`,
    md: `${SITE}/${fileNameOf(r)}`,
    note: pages.get(r).description,
  })),
};
const llmsStaticJson = JSON.stringify(llmsStatic, null, 2) + '\n';
if (leerTexto(LLMS_STATIC_FILE) !== llmsStaticJson) {
  writeFileSync(LLMS_STATIC_FILE, llmsStaticJson, 'utf8');
  console.log('  src/data/llms-static.json actualizado — commitealo con el cambio');
}


function urlEntry(route) {
  const { html } = pages.get(route);
  const images = rasterImages(html)
    .map(u => `    <image:image><image:loc>${SITE}${xml(u)}</image:loc></image:image>`)
    .join('\n');

  return [
    '  <url>',
    `    <loc>${SITE}${route}</loc>`,
    `    <lastmod>${fechaDe(route) || HOY}</lastmod>`,
    `    <changefreq>${route === '/' ? 'weekly' : 'yearly'}</changefreq>`,
    `    <priority>${route === '/' ? '1.0' : '0.3'}</priority>`,
    images,
    '  </url>',
  ].filter(Boolean).join('\n');
}

const sitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"
        xmlns:image="http://www.google.com/schemas/sitemap-image/1.1">
${INDEXED.filter(r => pages.has(r)).map(urlEntry).join('\n')}
</urlset>
`;

await writeFile(join(DIST, 'sitemap.xml'), sitemap, 'utf8');
console.log('  sitemap.xml');
/* ------------------------------------------------- headers de vercel.json */

/**
 * Que ninguna página quede sin su header `Link` en vercel.json.
 *
 * El sitemap y los gemelos en Markdown salen de recorrer dist/, así
 * que una página nueva entra sola. Los headers no: esa lista se escribe a mano,
 * y una página sin su entrada sirve el .md como HTML sin que nada falle. Ya
 * pasó dos veces, y las dos las encontró un test y no el build.
 *
 * Por eso esto rompe el build en vez de avisar. Corre dentro del buildCommand
 * de Vercel, así que una página sin header no llega a producción.
 *
 * `--fix-headers` escribe las que falten, para no tipearlas: se corre local,
 * se commitea el resultado. En Vercel no sirve, porque vercel.json ya se leyó
 * cuando este script arranca.
 */
const VERCEL = new URL('../vercel.json', import.meta.url);

function headerDe(route) {
  const md = fileNameOf(route);
  return {
    source: route,
    headers: [{
      key: 'Link',
      value: `<${SITE}/${md}>; rel="alternate"; type="text/markdown", `
        + `<${SITE}/llms.txt>; rel="describedby"; type="text/plain"`,
    }],
  };
}

const vercel = JSON.parse(await readFile(VERCEL, 'utf8'));
const declaradas = new Set(vercel.headers.map(h => h.source));
const faltantes = INDEXED.filter(r => !declaradas.has(r));

if (faltantes.length && process.argv.includes('--fix-headers')) {
  const i = vercel.headers.findLastIndex(h => h.source.startsWith('/guias'));
  vercel.headers.splice(i + 1, 0, ...faltantes.map(headerDe));
  await writeFile(VERCEL, JSON.stringify(vercel, null, 2) + '\n', 'utf8');
  console.log(`  vercel.json  ${faltantes.length} header(s) agregado(s): ${faltantes.join(', ')}`);
} else if (faltantes.length) {
  console.error(`\nvercel.json no declara el header Link de: ${faltantes.join(', ')}`);
  console.error('Corré `node scripts/build-markdown.mjs --fix-headers` y commiteá el cambio.\n');
  process.exit(1);
} else {
  console.log(`  vercel.json  ${INDEXED.length} header(s) declarado(s)`);
}

console.log(`[agents] ${pages.size} page(s), last modified ${HOY}`);
