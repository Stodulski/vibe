/**
 * La puerta SEO. Misma checklist, dos momentos.
 *
 *   node scripts/seo-gate.mjs --dist                    antes de deployar
 *   node scripts/seo-gate.mjs --url https://vibe.com.ar despues de deployar
 *
 * Son dos modos y no dos scripts porque la checklist es la misma: lo unico que
 * cambia es de donde sale el HTML. Y esa diferencia es justamente el punto.
 *
 * `--dist` mira lo que el build produjo. `--url` mira lo que el servidor
 * efectivamente devuelve, que no siempre es lo mismo: headers que pisan el
 * robots meta, un rewrite, una redireccion, una CDN sirviendo una version vieja.
 * Un gate que solo mira el codigo fuente aprueba cosas que en produccion estan
 * rotas.
 *
 * Sale con codigo 1 si algo no pasa. Nada de "warnings" que se aprenden a
 * ignorar: si esta en la lista es porque bloquea.
 *
 * Y bloquea de verdad: el modo --dist corre dentro del buildCommand de Vercel
 * (ver vercel.json), asi que una pagina que no pasa rompe el build y no llega a
 * produccion. Antes vivia en un script suelto que nadie estaba obligado a
 * correr, o sea que era una sugerencia con nombre de puerta.
 *
 * La contracara hay que tenerla presente: un bug ACA bloquea todos los deploys,
 * incluido el que vendria a arreglarlo. Si eso pasa, la salida es sacar el
 * `&& npm run seo:gate` del buildCommand, deployar el arreglo, y volverlo a
 * poner. No agregar excepciones adentro de las reglas.
 */
import { readFile, readdir } from 'node:fs/promises';
import { join, relative } from 'node:path';

const DIST = new URL('../dist/', import.meta.url).pathname;
const SITE = 'https://vibe.com.ar';

/* ------------------------------------------------------------------ reglas */

/** Menos que esto no es una pagina, es un placeholder. */
const MINIMO_DE_PALABRAS = 150;

/** Dos paginas que comparten mas que esto de su texto compiten entre si. */
const MAXIMO_DE_SOLAPAMIENTO = 0.6;

/* Google corta el title alrededor de los 600px y la description alrededor de
   los 920px. En caracteres es una aproximacion, pero sirve para agarrar los
   casos groseros: un title de 30 y uno de 120 son los dos un problema. */
const TITLE = { min: 20, max: 65 };
const DESCRIPTION = { min: 70, max: 165 };

/* ------------------------------------------------------------------ helpers */

const problemas = [];
const notas = [];
const fallar = (ruta, msg) => problemas.push({ ruta, msg });

const atributo = (html, re) => (html.match(re)?.[1] ?? '').trim();

const sinTags = (html) => html
  .replace(/<(script|style|noscript|svg)[^>]*>[\s\S]*?<\/\1>/gi, ' ')
  .replace(/<[^>]+>/g, ' ')
  .replace(/&[a-z]+;/gi, ' ')
  .replace(/\s+/g, ' ')
  .trim();

/** El <body>, para no contar el <head> como contenido. */
const cuerpo = (html) => atributo(html, /<body[^>]*>([\s\S]*)<\/body>/i) || html;

/**
 * Solo el contenido propio de la pagina: <main>, sin nav ni footer.
 *
 * Comparar paginas con el chrome adentro da falsos positivos garantizados. Nav,
 * footer y CTA son identicos en todo el sitio, asi que dos paginas que no tienen
 * nada que ver ya arrancan compartiendo cien palabras. Un gate que grita cuando
 * no pasa nada es un gate que se aprende a ignorar.
 */
const propio = (html) => {
  const main = atributo(html, /<main[^>]*>([\s\S]*?)<\/main>/i);
  return main || cuerpo(html).replace(/<(nav|footer)[^>]*>[\s\S]*?<\/\1>/gi, ' ');
};

function meta(html, nombre) {
  /* Astro emite los atributos alfabeticamente, asi que `content` cae antes que
     `name`. Se buscan los dos ordenes. */
  return atributo(html, new RegExp(`<meta[^>]*\\bcontent="([^"]*)"[^>]*\\bname="${nombre}"`, 'i'))
    || atributo(html, new RegExp(`<meta[^>]*\\bname="${nombre}"[^>]*\\bcontent="([^"]*)"`, 'i'));
}

const headings = (html, n) => [...html.matchAll(new RegExp(`<h${n}[^>]*>([\\s\\S]*?)</h${n}>`, 'gi'))]
  .map(m => sinTags(m[1]));

/** Enlaces internos, normalizados a ruta. */
function enlacesInternos(html) {
  const hrefs = [...cuerpo(html).matchAll(/<a[^>]*\bhref="([^"]+)"/gi)].map(m => m[1]);
  return [...new Set(hrefs
    .map(h => h.startsWith(SITE) ? h.slice(SITE.length) : h)
    .filter(h => h.startsWith('/') && !h.startsWith('//'))
    .map(h => h.split('#')[0].split('?')[0])
    .map(h => h.replace(/\/+$/, '') || '/')
    .filter(Boolean))];
}

const esIndexable = (html) => {
  const r = meta(html, 'robots');
  return !/noindex/i.test(r);
};

/**
 * Cuanto se pisan dos paginas, por Jaccard sobre su vocabulario propio.
 *
 * Jaccard y no "cuanto de la chica esta en la grande": esa segunda medida da
 * casi 1 siempre que una pagina sea corta, asi que un indice de dos lineas
 * apareceria canibalizando a la guia que lista. Que es exactamente lo contrario
 * de lo que pasa: el indice existe PARA mandar a la guia.
 *
 * Canibalizar es que dos paginas peleen por la misma intencion, y eso solo
 * puede pasar entre dos paginas que dicen mas o menos lo mismo con mas o menos
 * el mismo peso. Jaccard captura eso y el hub/hijo no.
 */
function solapamiento(a, b) {
  const bolsa = t => new Set(t.toLowerCase().match(/[a-záéíóúñü]{4,}/g) || []);
  const A = bolsa(a), B = bolsa(b);
  if (!A.size || !B.size) return 0;
  let comunes = 0;
  for (const w of A) if (B.has(w)) comunes++;
  return comunes / (A.size + B.size - comunes);
}

/* ------------------------------------------------------------- recoleccion */

async function htmlsDeDist(dir = DIST) {
  const out = [];
  for (const e of await readdir(dir, { withFileTypes: true })) {
    const full = join(dir, e.name);
    if (e.isDirectory()) out.push(...await htmlsDeDist(full));
    else if (e.name.endsWith('.html')) out.push(full);
  }
  return out;
}

function rutaDe(archivo) {
  const rel = relative(DIST, archivo).replace(/\\/g, '/');
  if (rel === 'index.html') return '/';
  return '/' + rel.replace(/\/index\.html$/, '').replace(/\.html$/, '');
}

async function leerDist() {
  const paginas = new Map();
  for (const f of await htmlsDeDist()) {
    paginas.set(rutaDe(f), { html: await readFile(f, 'utf8'), estado: 200, headers: {} });
  }
  const sitemap = await readFile(join(DIST, 'sitemap.xml'), 'utf8').catch(() => '');
  return { paginas, sitemap };
}

async function leerProduccion(base) {
  const sitemap = await (await fetch(`${base}/sitemap.xml`)).text();
  const enSitemap = [...sitemap.matchAll(/<loc>([^<]+)<\/loc>/g)]
    .map(m => m[1].replace(base, '').replace(/\/+$/, '') || '/');

  /* El sitemap solo trae lo indexable. Las paginas noindex tambien hay que
     mirarlas: son justamente donde un robots mal puesto no se nota. Se llega a
     ellas siguiendo los enlaces de las que si estan. */
  const paginas = new Map();
  const pendientes = [...new Set([...enSitemap, '/'])];

  while (pendientes.length) {
    const ruta = pendientes.shift();
    if (paginas.has(ruta)) continue;
    const res = await fetch(`${base}${ruta === '/' ? '/' : ruta}`, { redirect: 'manual' });
    const html = res.status < 300 ? await res.text() : '';
    paginas.set(ruta, {
      html,
      estado: res.status,
      headers: Object.fromEntries(res.headers),
    });
    if (html) for (const h of enlacesInternos(html)) {
      if (!paginas.has(h) && !pendientes.includes(h)) pendientes.push(h);
    }
  }
  return { paginas, sitemap };
}

/* ---------------------------------------------------------------- la puerta */

function revisar({ paginas, sitemap }, modo) {
  const enSitemap = new Set([...sitemap.matchAll(/<loc>([^<]+)<\/loc>/g)]
    .map(m => m[1].replace(SITE, '').replace(/\/+$/, '') || '/'));

  const titulos = new Map();
  const descripciones = new Map();
  const textos = new Map();
  const enlazadaDesde = new Map();

  for (const [ruta, p] of paginas) {
    if (p.estado !== 200) {
      fallar(ruta, `responde ${p.estado}, no 200`);
      continue;
    }
    for (const destino of enlacesInternos(p.html)) {
      if (destino === ruta) continue;
      if (!enlazadaDesde.has(destino)) enlazadaDesde.set(destino, []);
      enlazadaDesde.get(destino).push(ruta);
    }
  }

  for (const [ruta, p] of paginas) {
    if (p.estado !== 200) continue;
    const { html } = p;
    const indexable = esIndexable(html);

    /* --- title --- */
    const title = atributo(html, /<title[^>]*>([\s\S]*?)<\/title>/i);
    if (!title) fallar(ruta, 'no tiene <title>');
    else {
      if (titulos.has(title)) fallar(ruta, `repite el title de ${titulos.get(title)}`);
      else titulos.set(title, ruta);
      if (title.length < TITLE.min || title.length > TITLE.max) {
        fallar(ruta, `el title mide ${title.length} caracteres (se espera entre ${TITLE.min} y ${TITLE.max})`);
      }
    }

    /* --- meta description --- */
    const desc = meta(html, 'description');
    if (!desc) fallar(ruta, 'no tiene meta description');
    else {
      if (descripciones.has(desc)) fallar(ruta, `repite la description de ${descripciones.get(desc)}`);
      else descripciones.set(desc, ruta);
      if (desc.length < DESCRIPTION.min || desc.length > DESCRIPTION.max) {
        fallar(ruta, `la description mide ${desc.length} caracteres (se espera entre ${DESCRIPTION.min} y ${DESCRIPTION.max})`);
      }
    }

    /* --- H1 --- */
    const h1 = headings(html, 1);
    if (h1.length === 0) fallar(ruta, 'no tiene H1');
    else if (h1.length > 1) fallar(ruta, `tiene ${h1.length} H1: ${h1.join(' | ')}`);

    /* --- canonical --- */
    const canonical = atributo(html, /<link[^>]*\brel="canonical"[^>]*\bhref="([^"]+)"/i)
      || atributo(html, /<link[^>]*\bhref="([^"]+)"[^>]*\brel="canonical"/i);
    if (!canonical) fallar(ruta, 'no tiene canonical');
    else {
      if (!canonical.startsWith('http')) fallar(ruta, `el canonical es relativo: ${canonical}`);
      const esperado = `${SITE}${ruta === '/' ? '' : ruta}`;
      if (canonical.replace(/\/$/, '') !== esperado.replace(/\/$/, '')) {
        fallar(ruta, `el canonical apunta a ${canonical} y no a si misma`);
      }
    }

    /* --- robots --- */
    if (!meta(html, 'robots')) fallar(ruta, 'no declara meta robots');
    /* En produccion, un header puede pisar el meta sin que se note en la fuente. */
    const header = p.headers['x-robots-tag'] || '';
    if (indexable && /noindex/i.test(header)) {
      fallar(ruta, `el meta dice index pero el header X-Robots-Tag dice "${header}"`);
    }

    /* --- sitemap coherente con indexabilidad --- */
    const clave = ruta === '/' ? '/' : ruta;
    if (indexable && !enSitemap.has(clave)) fallar(ruta, 'es indexable y no esta en el sitemap');
    if (!indexable && enSitemap.has(clave)) fallar(ruta, 'es noindex y esta en el sitemap');

    /* --- contenido --- */
    const texto = sinTags(propio(html));
    textos.set(ruta, texto);
    const palabras = texto.split(/\s+/).filter(Boolean).length;
    if (indexable && palabras < MINIMO_DE_PALABRAS) {
      fallar(ruta, `tiene ${palabras} palabras, menos del minimo de ${MINIMO_DE_PALABRAS}`);
    }

    /* --- huerfanas --- */
    if (indexable && ruta !== '/' && !(enlazadaDesde.get(ruta) || []).length) {
      fallar(ruta, 'es indexable y no la enlaza ninguna otra pagina');
    }
  }

  /* --- canibalizacion --- */
  const indexables = [...paginas.entries()]
    .filter(([r, p]) => p.estado === 200 && esIndexable(p.html) && textos.has(r))
    .map(([r]) => r);
  for (let i = 0; i < indexables.length; i++) {
    for (let j = i + 1; j < indexables.length; j++) {
      const s = solapamiento(textos.get(indexables[i]), textos.get(indexables[j]));
      if (s > MAXIMO_DE_SOLAPAMIENTO) {
        fallar(indexables[i], `comparte ${(s * 100).toFixed(0)}% de su vocabulario con ${indexables[j]}: compiten por la misma intencion`);
      }
    }
  }

  notas.push(`${paginas.size} paginas revisadas en modo ${modo}`);
  /* En produccion se llega por sitemap y enlaces, asi que una pagina noindex a
     la que no enlaza nadie queda afuera. Se dice, en vez de dejar creer que la
     cobertura fue total. */
  if (modo.startsWith('produccion')) {
    notas.push('en este modo no se alcanzan las paginas noindex sin enlaces entrantes; para esas, usar --dist');
  }
  notas.push(`${indexables.length} indexables: ${indexables.sort().join(', ')}`);
}

/* -------------------------------------------------------------------- main */

const args = process.argv.slice(2);
const url = args.includes('--url') ? args[args.indexOf('--url') + 1] : null;

if (!url && !args.includes('--dist')) {
  console.error('Uso: node scripts/seo-gate.mjs --dist | --url https://vibe.com.ar');
  process.exit(2);
}

const datos = url ? await leerProduccion(url.replace(/\/$/, '')) : await leerDist();
revisar(datos, url ? `produccion (${url})` : 'dist');

notas.forEach(n => console.log(`  ${n}`));

if (problemas.length) {
  console.error(`\nLa puerta SEO rechaza ${problemas.length} cosa(s):\n`);
  for (const { ruta, msg } of problemas) console.error(`  ${ruta}\n    ${msg}`);
  console.error('\nNinguna de estas es opcional. Si alguna deberia serlo, hay que discutir la');
  console.error('regla en scripts/seo-gate.mjs, no dejarla pasar en silencio.');
  process.exit(1);
}

console.log('\nLa puerta SEO pasa.');
