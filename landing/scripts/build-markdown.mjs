/**
 * Post-build pass that produces everything an AI agent reads instead of the page:
 *
 *   dist/<route>.md   a Markdown twin of each page, served at the page's own URL
 *                     when the request carries `Accept: text/markdown`
 *                     (see middleware.ts — vercel.json rewrites cannot do this,
 *                     they are evaluated after the filesystem lookup)
 *   dist/llms.txt     the llmstxt.org index: what this site is, and where to read it
 *   dist/sitemap.xml  regenerated, each page carrying the date its own output
 *                     last changed (see src/data/page-dates.json)
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

const DIST = new URL('../dist/', import.meta.url).pathname;
const SITE = 'https://vibe.com.ar';

/* Which pages get an index entry is NOT a list anybody maintains. It used to be,
   and a hand-kept list is exactly the thing that goes stale the first time
   somebody adds a page and forgets this file: the page then exists, is indexable,
   and is missing from both the sitemap and llms.txt without anything failing.

   The page already declares it, in the one place that cannot disagree with what
   is served: its own robots meta. Anything that says noindex (404 and the like)
   is out; everything else is in. */
const esIndexable = html => !/<meta[^>]*\bname="robots"[^>]*\bcontent="[^"]*noindex/i.test(html)
  && !/<meta[^>]*\bcontent="[^"]*noindex[^"]*"[^>]*\bname="robots"/i.test(html);

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

/* What every real client downloads, which is what the sitemap should describe.
   The <img src> is only a fallback: browsers take the WebP from <source>, and
   Googlebot renders mobile-first, so it never sees the PNG at all. Both the
   mobile and desktop sources are listed because both are genuinely served,
   depending on the viewport. SVG is left out: image search does not index it. */
function rasterImages(html) {
  const fromSources = [...html.matchAll(/<source[^>]*\bsrcset="([^"]+)"/gi)]
    .map(m => m[1].split(',')[0].trim().split(/\s+/)[0]);
  const fromImgs = [...html.matchAll(/<img[^>]*\bsrc="([^"]+)"/gi)].map(m => m[1]);

  const all = [...fromSources, ...fromImgs].filter(u => /\.(png|jpe?g|webp)$/i.test(u));
  /* Drop a PNG when the same picture is also offered as WebP. */
  const stems = new Set(all.filter(u => /\.webp$/i.test(u)).map(u => u.replace(/\.webp$/i, '')));
  return [...new Set(all.filter(u => /\.webp$/i.test(u) || !stems.has(u.replace(/\.(png|jpe?g)$/i, ''))))];
}

/* Read the video's facts back out of the VideoObject already in the page rather
   than restating them here, so the two can never drift apart. */
function videoObject(html) {
  for (const m of html.matchAll(/<script type="application\/ld\+json"[^>]*>([\s\S]*?)<\/script>/gi)) {
    try {
      const d = JSON.parse(m[1]);
      if (d['@type'] === 'VideoObject') return d;
    } catch { /* not every block on the page has to parse */ }
  }
  return null;
}

const xml = s => String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');

/* "PT17S" -> 17, which is what a video sitemap wants. */
function isoSeconds(iso) {
  const m = /^PT(?:(\d+)M)?(?:(\d+(?:\.\d+)?)S)?$/.exec(iso || '');
  return m ? Math.round((+(m[1] || 0)) * 60 + (+(m[2] || 0))) : null;
}

function metaDescription(html) {
  /* Astro emits attributes alphabetically, so `content` lands before `name`. */
  return attr(html, /<meta[^>]*\bcontent="([^"]*)"[^>]*\bname="description"[^>]*>/i);
}

function toMarkdown(html, route) {
  const title = attr(html, /<title[^>]*>([\s\S]*?)<\/title>/i);
  const description = metaDescription(html);
  const body = attr(html, /<body[^>]*>([\s\S]*?)<\/body>/i) || html;

  const front = [
    '---',
    `title: ${JSON.stringify(title)}`,
    description && `description: ${JSON.stringify(description)}`,
    `canonical: ${SITE}${route}`,
    `lastModified: ${LAST_MODIFIED}`,
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

const LAST_MODIFIED = HOY;
const DATES = leerDates();

const pages = new Map();
for (const file of await htmlFiles(DIST)) {
  const route = routeOf(file);
  const html = await readFile(file, 'utf8');
  const { title, description, markdown } = toMarkdown(html, route);
  await writeFile(join(DIST, fileNameOf(route)), markdown, 'utf8');
  pages.set(route, { title, description, markdown, html });
  console.log(`  markdown  ${route} -> ${fileNameOf(route)}`);
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

const home = pages.get('/');
const summary = summaryFrom(home?.markdown ?? '', home?.description ?? '');

const llms = `# Vibe

> ${home?.description ?? ''}

${summary}

Toda página de este sitio responde en Markdown si la pedís con la cabecera
\`Accept: text/markdown\`, y en HTML si no. Los archivos \`.md\` listados abajo
son ese mismo contenido, accesible directamente.

Última actualización: ${LAST_MODIFIED}

## Páginas

${INDEXED.filter(r => pages.has(r)).map(r => {
  const p = pages.get(r);
  return `- [${p.title}](${SITE}${r}${r === '/' ? '' : ''}): ${p.description} — Markdown: ${SITE}/${fileNameOf(r)}`;
}).join('\n')}
`;

await writeFile(join(DIST, 'llms.txt'), llms, 'utf8');
console.log('  llms.txt');

/* A page keeps its stored date while its output is unchanged, and takes today's
   only when it actually differs. New pages start at today. */
let datesCambiaron = false;
for (const route of INDEXED) {
  if (!pages.has(route)) continue;
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

function urlEntry(route) {
  const { html } = pages.get(route);
  const images = rasterImages(html)
    .map(u => `    <image:image><image:loc>${SITE}${xml(u)}</image:loc></image:image>`)
    .join('\n');

  const v = videoObject(html);
  const seconds = v && isoSeconds(v.duration);
  const video = v ? `    <video:video>
      <video:thumbnail_loc>${xml(v.thumbnailUrl)}</video:thumbnail_loc>
      <video:title>${xml(v.name)}</video:title>
      <video:description>${xml(v.description)}</video:description>
      <video:content_loc>${xml(v.contentUrl)}</video:content_loc>${seconds ? `
      <video:duration>${seconds}</video:duration>` : ''}
      <video:publication_date>${xml(v.uploadDate)}</video:publication_date>
      <video:family_friendly>yes</video:family_friendly>
    </video:video>` : '';

  return [
    '  <url>',
    `    <loc>${SITE}${route}</loc>`,
    `    <lastmod>${DATES[route]?.date || HOY}</lastmod>`,
    `    <changefreq>${route === '/' ? 'weekly' : 'yearly'}</changefreq>`,
    `    <priority>${route === '/' ? '1.0' : '0.3'}</priority>`,
    images,
    video,
    '  </url>',
  ].filter(Boolean).join('\n');
}

const sitemap = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"
        xmlns:image="http://www.google.com/schemas/sitemap-image/1.1"
        xmlns:video="http://www.google.com/schemas/sitemap-video/1.1">
${INDEXED.filter(r => pages.has(r)).map(urlEntry).join('\n')}
</urlset>
`;

await writeFile(join(DIST, 'sitemap.xml'), sitemap, 'utf8');
console.log('  sitemap.xml');
console.log(`[agents] ${pages.size} page(s), last modified ${LAST_MODIFIED}`);
