/**
 * Functional smoke test for the deployed landing.
 *
 *   npm test                        # checks the deployed site
 *   node tests/smoke.mjs http://localhost:4321   # or a local `astro preview`
 *
 * Playwright SI es dependencia de desarrollo, y antes no lo era. La razon para
 * dejarlo afuera eran los varios cientos de MB de los navegadores, pero desde la
 * 1.62 el paquete no trae postinstall: `npm install` baja 19 MB de JavaScript y
 * ningun navegador. Los navegadores se bajan aparte, una sola vez:
 *
 *   npm run test:setup     (playwright install chromium, ~650 MB en cache)
 *
 * Mientras estuvo afuera, la suite andaba solo en la maquina donde alguien habia
 * enlazado playwright a mano desde otro repo. En un clon limpio no corria, que es
 * lo mismo que no tenerla.
 *
 * Vercel igual no la necesita: vercel.json pone PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD
 * por si una version futura vuelve a traer el postinstall.
 *
 * There is no test runner here on purpose. The point is to drive the real
 * deployed site with a real browser; a runner would add weight without adding
 * a single check.
 */
import { chromium, devices } from 'playwright';
import { guias } from '../src/data/guias.ts';
import {
  COSTOS, PLAZOS, porcentaje, IVA, RECARGO_TARJETA_EXTRANJERA,
  BRECHA_INMEDIATA, VIGENTE_DESDE, FUENTE,
} from '../src/data/mercadopago-costos.ts';
import { ejemplo, pesos, CARGO_SERVICIO, CARGO_MINIMO } from '../src/data/precio.ts';
import { faq } from '../src/data/faq.ts';
import { tutoriales, CDN } from '../src/data/tutoriales.ts';

const BASE = (process.argv[2] || 'https://vibe.com.ar').replace(/\/$/, '');
const results = [];
let browser;

function check(name, ok, detail = '') {
  results.push({ name, ok, detail });
  console.log(`  ${ok ? '✓' : '✗'} ${name}${detail ? '  — ' + detail : ''}`);
}

async function newPage(deviceName) {
  const ctx = await browser.newContext(
    deviceName ? devices[deviceName] : { viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  page.__errors = [];
  page.__failed = [];
  page.on('pageerror', e => page.__errors.push(e.message));
  page.on('requestfailed', r => {
    /* Analytics is loaded lazily and may be blocked; it is not the site's job. */
    if (!/googletagmanager|google-analytics/.test(r.url())) {
      page.__failed.push(r.url());
    }
  });
  page.on('response', r => {
    /* The document's own status is asserted by the caller; only sub-resources
       count as broken here, or the 404 page fails its own test. */
    if (r.status() >= 400 && new URL(r.url()).origin === BASE
        && r.request().resourceType() !== 'document') {
      page.__failed.push(`${r.status()} ${r.url()}`);
    }
  });
  return page;
}

/* ---------------------------------------------------------------- páginas */
async function testPages() {
  console.log('\nPÁGINAS');
  for (const [path, expected] of [['/', 200], ['/privacidad', 200], ['/terminos', 200],
                                  ['/ruta-inexistente', 404]]) {
    const p = await newPage();
    const res = await p.goto(BASE + path, { waitUntil: 'load' });
    check(`${path} responde ${expected}`, res.status() === expected, `recibido ${res.status()}`);
    check(`${path} sin errores de JS`, p.__errors.length === 0, p.__errors.join(' | ').slice(0, 90));
    check(`${path} sin recursos rotos`, p.__failed.length === 0, p.__failed.join(' | ').slice(0, 90));
    await p.context().close();
  }
}

/* ------------------------------------------------------------------ links */
async function testLinks() {
  console.log('\nENLACES');
  const p = await newPage();
  await p.goto(BASE + '/', { waitUntil: 'load' });

  const hrefs = await p.evaluate(() => [...new Set([...document.querySelectorAll('a[href]')]
      .map(a => a.getAttribute('href')))]);

  /* Se revisa en TODAS las paginas, no solo en la home. Un ancla suelta en
     /privacidad no lleva a ningun lado si esa seccion vive solo en la home.
     Revisando solo la home no se ve. */
  /* La lista NO se escribe a mano. Era ['/', '/privacidad', '/terminos', '/404']
     y cuando se agregaron /guias y las guias nadie se acordo de sumarlas: las
     dos salieron con cuatro anclas muertas en el footer y este chequeo, que
     existia y estaba bien, no las miraba.
     Ahora sale del sitemap mas las noindex conocidas, que no estan ahi. */
  const delSitemap = [...(await (await p.request.get(BASE + '/sitemap.xml')).text())
    .matchAll(/<loc>([^<]+)<\/loc>/g)]
    .map(m => m[1].replace(/^https:\/\/vibe\.com\.ar/, '').replace(/\/+$/, '') || '/');
  for (const path of [...delSitemap, '/404']) {
    await p.goto(BASE + path, { waitUntil: 'load' });
    const rotas = await p.evaluate(() => [...document.querySelectorAll('a[href^="#"]')]
      .map(a => a.getAttribute('href'))
      .filter(h => h !== '#' && !document.querySelector(h)));
    check(`${path}: las anclas apuntan a una sección de esta página`, rotas.length === 0, rotas.join(', '));
  }
  await p.goto(BASE + '/', { waitUntil: 'load' });

  const internal = hrefs.filter(h => h.startsWith('/') && !h.startsWith('//'));
  for (const href of internal) {
    const r = await p.request.get(BASE + href);
    check(`enlace interno ${href}`, r.status() === 200, `${r.status()}`);
  }

  /* Un 403 o un 400 al request pelado no significa que el enlace este roto:
     algunos hosts rechazan cualquier cliente que no parezca un navegador
     (MercadoPago contesta 403, Facebook 400). Antes de darlo por muerto se
     reintenta navegando de verdad, que es lo que hace la persona que hace
     click. Cualquier otro codigo >= 400 sigue fallando, y una navegacion que
     tampoco llega tambien. */
  const RECHAZO_A_BOTS = [400, 403];
  const external = hrefs.filter(h => /^https?:/.test(h));
  for (const href of external) {
    const host = new URL(href).host;
    let status = null;
    try {
      status = (await p.request.get(href, { timeout: 15000, maxRedirects: 5 })).status();
    } catch { /* sin respuesta al request; se decide abajo */ }

    if (status !== null && status < 400) {
      check(`enlace externo ${host}`, true, `${status}`);
      continue;
    }
    if (status !== null && !RECHAZO_A_BOTS.includes(status)) {
      check(`enlace externo ${host}`, false, `${status}`);
      continue;
    }

    /* El reintento va en un contexto con user agent de Chrome real. El resto de
       la suite corre con el UA por defecto de Playwright ("HeadlessChrome"), que
       es justo lo que estos hosts rechazan: reintentar con el mismo UA daria 403
       de nuevo y no probaria nada. */
    let navegado = null;
    try {
      const ctx = await browser.newContext(devices['Desktop Chrome']);
      const page = await ctx.newPage();
      navegado = (await page.goto(href, { waitUntil: 'domcontentloaded', timeout: 45000 }))?.status() ?? null;
      await ctx.close();
    } catch { /* tampoco navegando */ }

    check(
      `enlace externo ${host}`,
      navegado !== null && navegado < 400,
      navegado === null ? 'sin respuesta' : `${status ?? 'sin respuesta'} al request, ${navegado} navegando`,
    );
  }
  await p.context().close();
}

/* ------------------------------------------------------------ interacción */
async function testFaq() {
  console.log('\nFAQ');
  const p = await newPage();
  await p.goto(BASE + '/', { waitUntil: 'load' });
  await p.locator('#faq').scrollIntoViewIfNeeded();
  await p.waitForTimeout(600);

  const firstOpen = await p.evaluate(() => document.querySelector('.faq-item')?.open === true);
  check('la primera pregunta viene abierta', firstOpen);

  const second = p.locator('.faq-item').nth(1);
  await second.locator('summary').click();
  await p.waitForTimeout(600);
  const visible = await second.locator('p').evaluate(e => e.getBoundingClientRect().height > 20);
  check('al hacer clic se muestra la respuesta', visible);

  await second.locator('summary').click();
  await p.waitForTimeout(600);
  const collapsed = await second.locator('p').evaluate(e => e.getBoundingClientRect().height < 5);
  check('al volver a hacer clic se cierra', collapsed);
  await p.context().close();
}

async function testCarousel() {
  console.log('\nCARRUSEL');
  const p = await newPage();
  await p.goto(BASE + '/', { waitUntil: 'load' });
  await p.locator('#funcionalidades').scrollIntoViewIfNeeded();
  await p.waitForTimeout(800);

  const total = await p.locator('.ft-dot').count();
  check('hay puntos de progreso', total > 0, `${total} puntos`);

  const activeIndex = () => p.evaluate(() =>
    [...document.querySelectorAll('.ft-dot')].findIndex(d => d.classList.contains('active')));

  const start = await activeIndex();
  await p.locator('.ft-arrow-next').click();
  await p.waitForTimeout(900);
  check('la flecha siguiente avanza', await activeIndex() === (start + 1) % total);

  await p.locator('.ft-arrow-prev').click();
  await p.waitForTimeout(900);
  check('la flecha anterior retrocede', await activeIndex() === start);

  /* A full lap must land back on the first slide, not run off the end. */
  for (let i = 0; i < total; i++) {
    await p.locator('.ft-arrow-next').click();
    await p.waitForTimeout(750);
  }
  check('la vuelta completa vuelve al principio (loop infinito)', await activeIndex() === start,
        `terminó en ${await activeIndex()}`);

  await p.locator('.ft-dot').nth(3).click();
  await p.waitForTimeout(900);
  check('los puntos saltan a la slide correcta', await activeIndex() === 3);

  /* The active slide has to sit in the middle of the viewport, not off to a side. */
  const centred = await p.evaluate(() => {
    const el = document.querySelector('.ft-block.active');
    const vp = document.querySelector('.ft-viewport');
    if (!el || !vp) return false;
    const a = el.getBoundingClientRect(), b = vp.getBoundingClientRect();
    return Math.abs((a.left + a.width / 2) - (b.left + b.width / 2)) < 30;
  });
  check('la slide activa queda centrada', centred);
  await p.context().close();
}

async function testVideo() {
  console.log('\nVISUAL DEL HERO');
  const p = await newPage();
  await p.goto(BASE + '/', { waitUntil: 'load' });
  const v = await p.evaluate(() => {
    const el = document.querySelector('.hero-orbit');
    return el && {
      hidden: el.getAttribute('aria-hidden') === 'true',
      chips: el.querySelectorAll('.orbit-item').length,
      emojis: el.querySelectorAll('.orbit-item .sport-emoji').length,
    };
  });
  check('la órbita de deportes existe', !!v);
  check('es decorativo (aria-hidden)', v && v.hidden);
  check('renderiza los emojis de deporte', v && v.chips >= 6, v ? `chips=${v.chips}` : '');
  check('cada chip trae su emoji', v && v.emojis === v.chips, v ? `emojis=${v.emojis}` : '');
  await p.context().close();
}

async function testImages() {
  console.log('\nIMÁGENES');
  const p = await newPage();
  await p.goto(BASE + '/', { waitUntil: 'load' });
  /* Walk the page so every lazy image gets its turn. */
  await p.evaluate(async () => {
    for (let y = 0; y < document.body.scrollHeight; y += 400) {
      window.scrollTo(0, y);
      await new Promise(r => setTimeout(r, 90));
    }
  });
  await p.waitForTimeout(1500);
  const imgs = await p.evaluate(() => [...document.querySelectorAll('img')].map(i => ({
    src: i.currentSrc || i.src, ok: i.complete && i.naturalWidth > 0,
    lazy: i.loading === 'lazy', alt: i.alt, clone: !!i.closest('.ft-block[aria-hidden="true"]'),
  })));
  const broken = imgs.filter(i => !i.ok && !i.lazy);
  check('ninguna imagen ansiosa está rota', broken.length === 0, broken.map(i => i.src).join(', ').slice(0, 80));
  const noAlt = imgs.filter(i => !i.alt && !i.clone);
  check('toda imagen de contenido tiene alt', noAlt.length === 0, noAlt.map(i => i.src.split('/').pop()).join(', '));
  const webp = imgs.filter(i => /\.webp$/.test(i.src)).length;
  check('las capturas se sirven en WebP', webp > 0, `${webp} en WebP`);
  await p.context().close();
}

/* --------------------------------------------------------------- responsive */
async function testResponsive() {
  console.log('\nRESPONSIVE');
  for (const w of [320, 390, 768, 1024, 1440]) {
    const ctx = await browser.newContext({ viewport: { width: w, height: 800 } });
    const p = await ctx.newPage();
    await p.goto(BASE + '/', { waitUntil: 'load' });
    await p.waitForTimeout(700);
    const overflow = await p.evaluate(() =>
      document.documentElement.scrollWidth - document.documentElement.clientWidth);
    check(`${w}px sin scroll horizontal`, overflow <= 1, `sobra ${overflow}px`);
    await ctx.close();
  }
}

async function testMobileMenu() {
  console.log('\nMENÚ MOBILE');
  const p = await newPage('Pixel 7');
  await p.goto(BASE + '/', { waitUntil: 'load' });
  await p.waitForTimeout(500);
  await p.locator('.nav-toggle').click();
  await p.waitForTimeout(500);
  check('el menú abre', await p.locator('.mobile-menu').getAttribute('aria-hidden') === 'false');
  await p.locator('.mobile-menu a').first().click();
  await p.waitForTimeout(900);
  check('al elegir una opción se cierra', await p.locator('.mobile-menu').getAttribute('aria-hidden') === 'true');
  await p.context().close();
}

async function testComparisonTabs() {
  console.log('\nTABS DE COMPARACIÓN (mobile)');
  const p = await newPage('Pixel 7');
  await p.goto(BASE + '/', { waitUntil: 'load' });
  await p.locator('#comparacion').scrollIntoViewIfNeeded();
  await p.waitForTimeout(700);
  const tabs = p.locator('.comparison-tab');
  check('los tabs se ven en mobile', await tabs.first().isVisible());
  await tabs.nth(2).click();
  await p.waitForTimeout(500);
  check('al tocar un tab se marca activo', await tabs.nth(2).getAttribute('aria-selected') === 'true');
  const vista = await p.evaluate(() => {
    const celdas = [...document.querySelectorAll('.comparison-table td')].filter(c => c.offsetParent !== null);
    return { celdas: celdas.length, columnas: new Set(celdas.map(c => c.dataset.col)).size };
  });
  check('se muestra una sola columna por vez', vista.columnas === 1, `${vista.columnas} columnas`);
  check('la columna elegida muestra sus cinco criterios', vista.celdas === 5, `${vista.celdas} celdas`);

  /* The column separator belongs to the desktop layout. It survived on mobile for
     the second and third columns because the rule that draws it uses
     :not(:first-child) and a media query adds no specificity, so a plainer
     selector could not override it. Every tab has to be checked: the default one
     is the first column, which never had the border. */
  for (const [i, nombre] of [[0, 'Vibe'], [1, 'WhatsApp'], [2, 'ATC Sports']]) {
    await tabs.nth(i).click();
    await p.waitForTimeout(350);
    const bordes = await p.evaluate(() => [...new Set(
      [...document.querySelectorAll('.comparison-table th, .comparison-table td')]
        .filter(c => c.offsetParent !== null)
        .map(c => getComputedStyle(c).borderLeftWidth))]);
    check(`sin línea vertical en la columna ${nombre}`, bordes.every(b => b === '0px'), bordes.join(','));
  }

  /* The role checks belong on desktop: on a phone the tabs hide two of the three
     columns, so only one column header is exposed and that is correct. */
  const d = await newPage();
  await d.goto(BASE + '/', { waitUntil: 'load' });
  await d.locator('#comparacion').scrollIntoViewIfNeeded();
  await d.waitForTimeout(600);
  check('la comparación es una tabla accesible', await d.getByRole('table').count() === 1);
  check('expone 6 filas (cabecera + 5 criterios)', await d.getByRole('row').count() === 6,
        `${await d.getByRole('row').count()}`);
  check('expone 3 encabezados de columna', await d.getByRole('columnheader').count() === 3,
        `${await d.getByRole('columnheader').count()}`);
  check('expone las 15 celdas', await d.getByRole('cell').count() === 15,
        `${await d.getByRole('cell').count()}`);
  await d.context().close();
  await p.context().close();
}

/* ------------------------------------------------------------------ agentes */
async function testAgents() {
  console.log('\nCONTENIDO PARA AGENTES');
  const p = await newPage();
  for (const path of ['/', '/privacidad', '/terminos']) {
    const md = await p.request.get(BASE + path, { headers: { accept: 'text/markdown' } });
    check(`${path} negocia markdown`, (md.headers()['content-type'] || '').includes('text/markdown'),
          md.headers()['content-type']);
    const html = await p.request.get(BASE + path, { headers: { accept: 'text/html' } });
    check(`${path} sigue dando HTML por defecto`, (html.headers()['content-type'] || '').includes('text/html'));
  }
  for (const f of ['/llms.txt', '/sitemap.xml', '/robots.txt']) {
    const r = await p.request.get(BASE + f);
    check(`${f} disponible`, r.status() === 200, `${r.status()}`);
  }
  await p.context().close();
}

/* Un contacto que aparece en la landing pero no en las paginas legales, o que la
   pagina dice distinto del JSON-LD, es el mismo error que ya nos mordio con el
   cargo de servicio: dos fuentes de verdad que se separan sin que nadie avise. */
const EMAIL = 'hola@vibe.com.ar';
const TEL_LEGIBLE = '+54 9 11 2463-8281';
const TEL_DIGITOS = '5491124638281';
/* Contactos retirados: si alguno reaparece, alguien revirtio a medias. */
const VIEJOS = ['contacto@vibe.com.ar', '5491151065935'];

async function testContacto() {
  console.log('\nCONTACTO');
  const p = await newPage();

  for (const path of ['/', '/privacidad', '/terminos', '/404']) {
    await p.goto(BASE + path, { waitUntil: 'load' });
    const f = await p.evaluate(() => {
      const c = document.querySelector('.footer-contact');
      if (!c) return null;
      return [...c.querySelectorAll('a')].map(a => ({
        texto: a.textContent.trim().replace(/\s+/g, ' '),
        href: a.getAttribute('href'),
      }));
    });
    /* Este corre siempre, tambien cuando la tira falta: un contacto retirado que
       sobrevive en el HTML es un bug aunque el resto del footer este bien. */
    const html = await p.content();
    for (const viejo of VIEJOS) {
      check(`${path} sin el contacto viejo ${viejo}`, !html.includes(viejo));
    }

    check(`${path} tiene la tira de contacto`, Array.isArray(f) && f.length === 2, JSON.stringify(f));
    if (!f) continue;
    check(`${path} mail correcto`,
          f.some(a => a.texto === EMAIL && a.href === `mailto:${EMAIL}`), JSON.stringify(f[0]));
    check(`${path} whatsapp correcto`,
          f.some(a => a.texto === TEL_LEGIBLE && a.href === `https://wa.me/${TEL_DIGITOS}`), JSON.stringify(f[1]));
  }

  /* El boton flotante es fixed: si tapa un link del footer, ese link no existe. */
  for (const [w, h] of [[390, 844], [360, 800], [320, 720]]) {
    await p.setViewportSize({ width: w, height: h });
    await p.goto(BASE + '/', { waitUntil: 'load' });
    /* Un scrollTo con espera fija miente dos veces: html tiene scroll-behavior:smooth,
       asi que el salto se anima, y la pagina crece mientras se baja porque entran las
       imagenes lazy y las animaciones. Medir ahi da un solape que no existe. Hay que
       insistir hasta que la altura y la posicion dejen de moverse las dos. */
    await p.evaluate(async () => {
      document.documentElement.style.scrollBehavior = 'auto';
      const frame = () => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r)));
      let alto = -1, y = -1, vueltas = 0;
      while (vueltas < 120) {
        window.scrollTo(0, document.body.scrollHeight);
        await frame();
        const nAlto = document.body.scrollHeight, nY = Math.round(window.scrollY);
        if (nAlto === alto && nY === y) break;
        alto = nAlto; y = nY; vueltas++;
      }
    });
    await p.waitForTimeout(300);
    const r = await p.evaluate(() => {
      const fab = document.querySelector('.whatsapp-fab');
      const links = [...document.querySelectorAll('.footer-contact a')];
      if (!fab) return { error: 'no hay boton flotante' };
      if (!links.length) return { error: 'no hay tira de contacto que comparar' };
      const f = fab.getBoundingClientRect();
      return { revisados: links.length,
        alFinal: Math.round(window.scrollY + window.innerHeight) >= document.body.scrollHeight - 2,
        tapados: links
          .filter(a => { const b = a.getBoundingClientRect();
            return !(f.right < b.left || f.left > b.right || f.bottom < b.top || f.top > b.bottom); })
          .map(a => a.textContent.trim().replace(/\s+/g, ' ')) };
    });
    check(`${w}px el boton de whatsapp no tapa el contacto`,
          !r.error && r.alFinal && r.revisados === 2 && r.tapados.length === 0, JSON.stringify(r));
  }
  await p.setViewportSize({ width: 1280, height: 900 });

  /* La pagina y el JSON-LD tienen que decir lo mismo. */
  await p.goto(BASE + '/', { waitUntil: 'load' });
  const org = await p.evaluate(() => {
    for (const s of document.querySelectorAll('script[type="application/ld+json"]')) {
      try { const j = JSON.parse(s.textContent); if (j['@type'] === 'Organization') return j; } catch {}
    }
    return null;
  });
  check('el JSON-LD Organization existe', !!org);
  if (org) {
    check('JSON-LD: mismo mail que la pagina', org.email === EMAIL, org.email);
    check('JSON-LD: mismo telefono que la pagina',
          org.telephone === `+${TEL_DIGITOS}`, org.telephone);
    check('JSON-LD: contactPoint coherente',
          org.contactPoint?.email === EMAIL && org.contactPoint?.telephone === `+${TEL_DIGITOS}`,
          JSON.stringify(org.contactPoint));
  }

  /* Los agentes leen el .md, no el HTML: ahi tambien tiene que estar. */
  for (const path of ['/', '/privacidad', '/terminos']) {
    const md = await (await p.request.get(BASE + path, { headers: { accept: 'text/markdown' } })).text();
    check(`${path}.md trae el mail`, md.includes(EMAIL));
    check(`${path}.md trae el whatsapp`, md.includes(TEL_DIGITOS));
  }
  await p.context().close();
}

/* Vibe ya esta abierto. Todo CTA manda a crear cuenta en la app, no a una
   lista de espera que ya no existe: ni la seccion, ni el formulario, ni
   /api/lista, ni la pagina /gracias que confirmaba el alta. */
async function testRegistro() {
  console.log('\nREGISTRO');
  const p = await newPage();

  const delSitemap = [...(await (await p.request.get(BASE + '/sitemap.xml')).text())
    .matchAll(/<loc>([^<]+)<\/loc>/g)]
    .map(m => m[1].replace(/^https:\/\/vibe\.com\.ar/, '').replace(/\/+$/, '') || '/');
  const paginas = [...delSitemap, '/404'];

  /* Barrido en TODAS las paginas, no solo la home: un `#lista` suelto en
     cualquier lado es un link muerto ahora que la seccion no existe. */
  for (const path of paginas) {
    await p.goto(BASE + path, { waitUntil: 'load' });
    const sueltos = await p.evaluate(() => [...document.querySelectorAll('a[href]')]
      .map(a => a.getAttribute('href'))
      .filter(h => h.includes('#lista')));
    check(`${path} sin ningun href a #lista`, sueltos.length === 0, sueltos.join(', '));
  }

  /* Ni la pagina ni su gemela en Markdown pueden seguir diciendo que Vibe
     no abrio: eso es lo unico que un check nuevo tiene que poder hacer
     fallar contra el markup viejo. */
  for (const path of paginas) {
    const html = await (await p.request.get(BASE + path)).text();
    check(`${path} sin "todavía no está abierto"`, !html.includes('todavía no está abierto'));
    check(`${path} sin "Todavía no abrimos"`, !html.includes('Todavía no abrimos'));
  }
  for (const path of ['/', '/privacidad', '/terminos']) {
    const md = await (await p.request.get(BASE + path, { headers: { accept: 'text/markdown' } })).text();
    check(`${path}.md sin "todavía no está abierto"`, !md.includes('todavía no está abierto'));
  }

  await p.goto(BASE + '/', { waitUntil: 'load' });
  const d = await p.evaluate(() => ({
    alRegistro: [...document.querySelectorAll('a[href="https://app.vibe.com.ar/register"]')]
      .map(a => a.textContent.trim()),
    ingresar: [...document.querySelectorAll('a[href="https://app.vibe.com.ar/login"]')]
      .map(a => a.textContent.trim()),
    empezar: !!document.querySelector('#empezar'),
  }));
  /* Hero, nav de escritorio, nav movil, Precio y el CTA final: 5 caminos a crear cuenta. */
  check('la home tiene 5 CTA hacia app.vibe.com.ar/register', d.alRegistro.length === 5, JSON.stringify(d.alRegistro));
  check('la nav tiene "Ingresar" en escritorio y en el menu movil', d.ingresar.length === 2, JSON.stringify(d.ingresar));
  check('la seccion #empezar existe', d.empezar === true);

  /* Las paginas legales ya no llevan el aviso de app cerrada. */
  for (const path of ['/privacidad', '/terminos']) {
    await p.goto(BASE + path, { waitUntil: 'load' });
    const l = await p.evaluate(() => ({
      aviso: !!document.querySelector('.legal-aviso'),
      fecha: document.querySelector('.legal-date time')?.getAttribute('datetime'),
      texto: document.body.textContent.replace(/\s+/g, ' '),
    }));
    check(`${path} sin .legal-aviso`, l.aviso === false);
    /* La fecha vieja quedo cinco meses atras mientras el sitio se reescribia. */
    check(`${path} no quedo con la fecha vieja`, l.fecha !== '2026-03-16', l.fecha);
    if (path === '/privacidad') {
      check('la politica declara los correos de la lista de espera', l.texto.includes('Correos de la lista de espera'));
      check('la politica declara donde se guardaron', l.texto.includes('Google Sheets'));
      check('la politica dice como pedir la baja', l.texto.includes('hola@vibe.com.ar'));
    }
  }

  /* /gracias confirmaba el alta en la lista. Sin lista, sin pagina. */
  const gr = await p.request.get(BASE + '/gracias', { failOnStatusCode: false });
  check('/gracias devuelve 404', gr.status() === 404, `${gr.status()}`);
  const sm = await (await p.request.get(BASE + '/sitemap.xml')).text();
  check('/gracias no esta en el sitemap', !sm.includes('/gracias'));

  const home = await (await p.request.get(BASE + '/', { headers: { accept: 'text/markdown' } })).text();
  check('/index.md manda a crear la cuenta', home.includes('https://app.vibe.com.ar/register'));
  const llms = await (await p.request.get(BASE + '/llms.txt')).text();
  check('llms.txt no dice que sigue cerrada', !llms.includes('Todavía no está abierta al público'));
  check('llms.txt trae el link de registro', llms.includes('https://app.vibe.com.ar/register'));

  await p.context().close();
}

/* ------------------------------------------------------------------ guias */

/**
 * Las guias son paginas de dato, y un dato que se contradice con su fuente vale
 * menos que ninguno. Por eso lo que se comprueba no es que la tabla exista sino
 * que diga exactamente lo que dice src/data/mercadopago-costos.ts, que es la
 * transcripcion de la pagina oficial de MercadoPago.
 */
async function testGuias() {
  const p = await newPage();

  /* El indice tiene que enlazar todas: una guia a la que no se llega navegando
     depende del sitemap y nada mas. */
  const res = await p.goto(BASE + '/guias', { waitUntil: 'load' });
  check('/guias responde', res.status() === 200, `${res.status()}`);
  const enIndice = await p.evaluate(() => [...document.querySelectorAll('.guia-index-item a')]
    .map(a => new URL(a.href).pathname));
  for (const g of guias) {
    check(`/guias enlaza ${g.slug}`, enIndice.includes(`/guias/${g.slug}`));
  }

  const sitemap = await (await p.request.get(BASE + '/sitemap.xml')).text();
  const llms = await (await p.request.get(BASE + '/llms.txt')).text();

  for (const g of guias) {
    const url = `/guias/${g.slug}`;
    const r = await p.goto(BASE + url, { waitUntil: 'load' });
    check(`${url} responde`, r.status() === 200, `${r.status()}`);

    const visto = await p.evaluate(() => ({
      h1: document.querySelectorAll('h1').length,
      titulo: document.querySelector('h1')?.textContent.trim(),
      canonical: document.querySelector('link[rel=canonical]')?.href,
      robots: document.querySelector('meta[name=robots]')?.content,
      /* La respuesta tiene que estar antes del primer h2: si quedo despues del
         desarrollo, la pagina dejo de ser answer-first y es solo un articulo. */
      respuestaPrimero: (() => {
        const r = document.querySelector('.guia-respuesta');
        const h2 = document.querySelector('h2');
        return !!r && !!h2 && !!(r.compareDocumentPosition(h2) & Node.DOCUMENT_POSITION_FOLLOWING);
      })(),
      respuesta: document.querySelector('.guia-respuesta')?.textContent.trim(),
      preguntas: [...document.querySelectorAll('.guia-faq-item summary')].map(s => s.textContent.trim()),
    }));

    check(`${url} tiene un solo h1`, visto.h1 === 1, `${visto.h1}`);
    check(`${url} el h1 es el titulo de la guia`, visto.titulo === g.title, visto.titulo);
    check(`${url} tiene canonical propio`, visto.canonical === `https://vibe.com.ar${url}`, visto.canonical);
    check(`${url} es indexable`, /index/.test(visto.robots || '') && !/noindex/.test(visto.robots || ''), visto.robots);
    check(`${url} responde antes de desarrollar`, visto.respuestaPrimero);
    check(`${url} la respuesta es la del dato`, visto.respuesta === g.respuesta);
    check(`${url} publica sus ${g.faq.length} preguntas`,
      g.faq.every(f => visto.preguntas.includes(f.question)),
      `${visto.preguntas.length} en la pagina`);

    check(`${url} esta en el sitemap`, sitemap.includes(`https://vibe.com.ar${url}<`));
    check(`${url} esta en llms.txt`, llms.includes(`https://vibe.com.ar${url})`));

    /* El .md es lo que lee un agente, y la tabla es el motivo por el que la
       leeria. Turndown no sabe de tablas: sin regla propia se deshace en lineas
       sueltas donde ya no se sabe que fila es cual. */
    const md = await (await p.request.get(BASE + url, { headers: { accept: 'text/markdown' } })).text();
    check(`${url}.md llega como markdown`, md.startsWith('---'), md.slice(0, 30));
    check(`${url}.md no pego la fecha con la fuente`, !/\d{4}-\d{2}-\d{2}Fuente/.test(md));
  }

  await p.context().close();
}

/**
 * Los porcentajes viven en un solo archivo a proposito. Esto comprueba que la
 * home y la guia sigan mostrando ese archivo y no una copia que alguien edito
 * a mano, que es como quedaron mal la primera vez.
 */
async function testCostosMercadoPago() {
  const p = await newPage();

  await p.goto(BASE + '/guias/cuanto-cobra-mercadopago-por-una-sena', { waitUntil: 'load' });
  const tabla = await p.evaluate(() => [...document.querySelectorAll('.guia-tabla tbody tr')]
    .map(tr => [...tr.querySelectorAll('th, td')].map(c => c.textContent.trim())));

  check('la tabla tiene los 9 grupos de tarifas', tabla.length === COSTOS.length, `${tabla.length}`);
  COSTOS.forEach((grupo, i) => {
    const esperado = [grupo.provincias.join(', '), ...grupo.tasas.map(porcentaje)];
    check(`tarifa de ${grupo.provincias[0]} coincide con la fuente`,
      JSON.stringify(tabla[i]) === JSON.stringify(esperado),
      (tabla[i] || []).join(' '));
  });

  /* El detalle por plazo y provincia vive en la guia, no en la home: la home
     solo tiene que enlazarla y no repetir la tabla ni el ejemplo. */
  await p.goto(BASE + '/#precio', { waitUntil: 'load' });
  const link = await p.evaluate(() =>
    document.querySelector('a[href="/guias/cuanto-cobra-mercadopago-por-una-sena"]')?.textContent.trim());
  check('la home enlaza la guia de costos de MercadoPago', !!link, link ?? 'no esta');
  check('la home ya no repite la tabla de tarifas por plazo',
    await p.locator('.mp-fees-list').count() === 0);
  check('la home ya no repite el ejemplo con montos',
    await p.locator('.mp-fees-example').count() === 0);

  await p.context().close();
}

/**
 * El JSON en /datos/ es lo que la app (otro origen, app.vibe.com.ar) lee en vez
 * de repetir estos números a mano en su propio repositorio. Un JSON que exista
 * pero diga otra cosa que el dataset es peor que no tenerlo: la app confiaría
 * en un número que la landing ya no muestra.
 */
async function testDatosMercadoPagoJson() {
  console.log('\nDATOS MERCADOPAGO (JSON)');
  const p = await newPage();

  const res = await p.request.get(BASE + '/datos/mercadopago-costos.json');
  check('/datos/mercadopago-costos.json responde 200', res.status() === 200, `${res.status()}`);
  if (res.status() !== 200) { await p.context().close(); return; }

  const headers = res.headers();
  check('/datos/mercadopago-costos.json es application/json',
    (headers['content-type'] || '').includes('application/json'), headers['content-type']);
  check('/datos/mercadopago-costos.json tiene CORS abierto',
    headers['access-control-allow-origin'] === '*', headers['access-control-allow-origin']);

  let datos = null;
  try { datos = JSON.parse(await res.text()); } catch { /* se reporta abajo */ }
  check('/datos/mercadopago-costos.json es JSON válido', datos !== null);
  if (!datos) { await p.context().close(); return; }

  check('trae los 4 plazos del dataset, en orden',
    JSON.stringify(datos.plazos) === JSON.stringify(PLAZOS), JSON.stringify(datos.plazos));
  check('trae los 9 grupos del dataset', Array.isArray(datos.grupos) && datos.grupos.length === COSTOS.length,
    `${datos.grupos?.length}`);
  check('cada grupo trae sus 4 tasas',
    Array.isArray(datos.grupos) && datos.grupos.every(g => Array.isArray(g.tasas) && g.tasas.length === PLAZOS.length));
  check('vigente_desde es una fecha ISO', /^\d{4}-\d{2}-\d{2}$/.test(datos.vigente_desde), datos.vigente_desde);
  check('vigente_desde coincide con el dataset', datos.vigente_desde === VIGENTE_DESDE, datos.vigente_desde);
  check('fuente coincide con el dataset', datos.fuente === FUENTE, datos.fuente);
  check('iva_incluido es false', datos.iva_incluido === false, `${datos.iva_incluido}`);
  check('generado es una fecha parseable', !Number.isNaN(Date.parse(datos.generado)), datos.generado);

  /* Los grupos, uno por uno: no alcanza con contarlos, tienen que decir lo
     mismo que src/data/mercadopago-costos.ts, provincia por provincia. */
  const esperado = COSTOS.map(g => ({ provincias: g.provincias, tasas: g.tasas }));
  check('los grupos coinciden con el dataset, en el mismo orden',
    JSON.stringify(datos.grupos) === JSON.stringify(esperado),
    JSON.stringify(datos.grupos).slice(0, 120));

  /* El JSON y la guia no pueden decir dos cosas distintas del mismo número: la
     tasa de "Acreditación inmediata" del primer grupo tiene que ser la misma.
     La home ya no repite esta tabla, asi que la comparacion se hace contra la
     guia, que es donde vive el detalle por plazo y provincia. */
  await p.goto(BASE + '/guias/cuanto-cobra-mercadopago-por-una-sena', { waitUntil: 'load' });
  const primeraTasaGuia = await p.evaluate(() =>
    document.querySelector('.guia-tabla tbody tr:first-child td:first-child')?.textContent.trim());
  const primeraTasaJson = porcentaje(datos.grupos[0].tasas[0]);
  check('la primera tasa del JSON es la "Acreditación inmediata" que muestra la guia',
    primeraTasaGuia === primeraTasaJson, `${primeraTasaGuia} vs ${primeraTasaJson}`);

  await p.context().close();
}

/**
 * El invariante que cierra el problema de fondo.
 *
 * Los porcentajes y los montos estaban escritos a mano en seis lugares, y dos de
 * ellos ya habian quedado mal. Derivarlos de un dataset arregla los seis, pero no
 * impide que manana alguien escriba uno nuevo a mano en una pagina nueva.
 *
 * Esto barre TODO numero que parezca plata en las paginas que hablan de plata, y
 * exige que cada uno se pueda derivar de src/data/. Si aparece uno que no sale de
 * ahi, falla, sin importar de que pagina venga ni quien lo escribio.
 */
async function testNingunNumeroSuelto() {
  const p = await newPage();

  /* Todo lo que legitimamente puede aparecer, calculado, nunca tipeado. */
  const e = ejemplo();
  const porcentajesValidos = new Set([
    ...COSTOS.flatMap(g => g.tasas).map(porcentaje),
    `${CARGO_SERVICIO * 100}%`.replace('.', ','),
    `${IVA}%`,
    `${BRECHA_INMEDIATA.toFixed(2).replace('.', ',')}%`,
    '0%',
    `${RECARGO_TARJETA_EXTRANJERA}%`,
  ]);
  const pesosValidos = new Set([
    e.sena, e.cargo, e.totalCliente,
    e.inmediata.descuento, e.inmediata.neto,
    e.masLenta.descuento, e.masLenta.neto,
    CARGO_MINIMO, 0,
  ].map(pesos));

  for (const ruta of ['/', '/guias/cuanto-cobra-mercadopago-por-una-sena']) {
    await p.goto(BASE + ruta, { waitUntil: 'load' });
    const texto = await p.evaluate(() => document.body.innerText);

    const porcentajes = [...new Set([...texto.matchAll(/(\d+,\d+)\s*%/g)].map(m => `${m[1]}%`))];
    const sueltos = porcentajes.filter(x => !porcentajesValidos.has(x));
    check(`${ruta} no muestra ningun porcentaje fuera del dataset`,
      sueltos.length === 0, sueltos.join(' '));

    /* Solo montos con separador de miles: los sueltos de una cifra son precios
       de canchas inventados en otros ejemplos, no cuentas de comisiones. */
    const montos = [...new Set([...texto.matchAll(/\$\d{1,3}(?:\.\d{3})+/g)].map(m => m[0]))];
    const montosSueltos = montos.filter(x => !pesosValidos.has(x));
    check(`${ruta} no muestra ningun monto fuera de la cuenta`,
      montosSueltos.length === 0, montosSueltos.join(' '));
  }

  /* La FAQ visible y su schema salen de la misma fuente. Si alguien vuelve a
     escribir una de las dos aparte, Google descarta el markup entero. */
  await p.goto(BASE + '/', { waitUntil: 'load' });
  const visibles = await p.evaluate(() => [...document.querySelectorAll('.faq-item')]
    .map(d => ({
      q: d.querySelector('summary').textContent.trim(),
      a: d.querySelector('p').textContent.trim(),
    })));
  const schema = await p.evaluate(() => {
    for (const s of document.querySelectorAll('script[type="application/ld+json"]')) {
      const d = JSON.parse(s.textContent);
      if (d['@type'] === 'FAQPage') {
        return d.mainEntity.map(e => ({ q: e.name, a: e.acceptedAnswer.text }));
      }
    }
    return [];
  });

  check('la FAQ visible tiene las mismas preguntas que faq.ts', visibles.length === faq.length,
    `${visibles.length} vs ${faq.length}`);
  check('el FAQPage tiene las mismas que la FAQ visible', schema.length === visibles.length,
    `${schema.length} vs ${visibles.length}`);
  const desalineadas = visibles.filter((v, i) => v.q !== schema[i]?.q || v.a !== schema[i]?.a);
  check('cada respuesta del schema es textual la que se ve', desalineadas.length === 0,
    desalineadas.map(d => d.q).join(' | '));

  await p.context().close();
}

/**
 * Que dos enlaces vecinos no queden pegados.
 *
 * Pasa cuando dos <a> comparten un <li>: el gap de la lista separa items, no
 * hermanos dentro de un item, y compressHTML se lleva el espacio en blanco que
 * los separaba en el markup. El resultado es "GuíasPrivacidad", un enlace que
 * parece uno solo y una palabra que no existe.
 *
 * No se puede ver con una regex sobre el HTML: ahi los dos <a> estan bien
 * formados y separados. Solo aparece en el innerText, que es lo que lee una
 * persona. Por eso esto va contra el navegador.
 */
async function testEnlacesPegados() {
  const p = await newPage();
  await p.goto(BASE + '/', { waitUntil: 'load' });

  for (const zona of ['nav', 'footer']) {
    const pegados = await p.evaluate((sel) => {
      const raiz = document.querySelector(sel);
      if (!raiz) return ['no existe ' + sel];
      const texto = raiz.innerText;
      const links = [...raiz.querySelectorAll('a')]
        .map(a => a.innerText.trim()).filter(Boolean);
      const mal = [];
      for (let i = 0; i < links.length - 1; i++) {
        if (texto.includes(links[i] + links[i + 1])) mal.push(links[i] + '+' + links[i + 1]);
      }
      return mal;
    }, zona);
    check(`los enlaces del ${zona} no quedan pegados entre si`,
      pegados.length === 0, pegados.join(' '));
  }

  /* El link a las guias tiene que existir y llevar a alguna parte: es de donde
     salen todas, y sin el dependen del sitemap y nada mas. */
  const guias = await p.evaluate(() => {
    const a = [...document.querySelectorAll('footer a')].find(a => a.getAttribute('href') === '/guias');
    return a ? a.innerText.trim() : null;
  });
  check('el footer enlaza a /guias', guias === 'Guías', guias ?? 'no esta');

  await p.context().close();
}

/**
 * Que Google no corte el title ni la description.
 *
 * Google trunca por ANCHO EN PIXELES, no por cantidad de caracteres: ~600px con
 * Arial 20 para el title y ~920px con Arial 14 para la description. En castellano
 * la diferencia importa, porque las tildes y las letras anchas mueven el ancho
 * sin mover el conteo.
 *
 * Esto no es teorico. La guia de MercadoPago pasaba el limite de caracteres de
 * scripts/seo-gate.mjs (64 de 65) y aun asi medía 607px de title y 999px de
 * description: Google la cortaba de los dos lados y el conteo no lo mostraba.
 *
 * Vive aca y no en el gate porque medir pixeles necesita un canvas, o sea un
 * navegador, y el gate corre dentro del build de Vercel donde no hay ninguno.
 * El gate se queda con los caracteres como red gruesa; el ancho real se mide aca.
 */
async function testAnchoEnSerp() {
  const p = await newPage();
  /* Una pagina en blanco: lo unico que hace falta es el canvas para medir. */
  await p.setContent('<body></body>');

  const sitemap = await (await p.request.get(BASE + '/sitemap.xml')).text();
  const rutas = [...sitemap.matchAll(/<loc>([^<]+)<\/loc>/g)]
    .map(m => m[1].replace(/^https:\/\/vibe\.com\.ar/, '').replace(/\/+$/, '') || '/');

  for (const ruta of rutas) {
    const html = await (await p.request.get(BASE + ruta)).text();
    const title = (html.match(/<title[^>]*>([\s\S]*?)<\/title>/i) || [])[1] || '';
    const desc = (html.match(/<meta[^>]*\bcontent="([^"]*)"[^>]*\bname="description"/i) || [])[1] || '';

    const [anchoTitle, anchoDesc] = await p.evaluate(([t, d]) => {
      const ctx = document.createElement('canvas').getContext('2d');
      ctx.font = '20px Arial';
      const a = ctx.measureText(t).width;
      ctx.font = '14px Arial';
      return [Math.round(a), Math.round(ctx.measureText(d).width)];
    }, [title, desc]);

    check(`${ruta} el title entra en la SERP`, anchoTitle <= 600, `${anchoTitle}px de 600`);
    check(`${ruta} la description entra en la SERP`, anchoDesc <= 920, `${anchoDesc}px de 920`);
  }

  await p.context().close();
}

/* --------------------------------------------------- assets de los tutoriales */

/**
 * Que el video y el poster de cada tutorial existan de verdad en el CDN.
 *
 * Nada del build puede cachear esto: los archivos viven en otro host, así que
 * `astro build` compone la URL y da por buena cualquier cadena. Se rompió una
 * vez, en producción, porque el slug de la página y el nombre del archivo
 * subido dejaron de coincidir en dos de siete y nadie tenía cómo notarlo.
 */
async function testAssetsDeTutoriales() {
  const p = await newPage();
  for (const t of tutoriales) {
    for (const [tipo, ext] of [['poster', 'webp'], ['video', 'mp4']]) {
      const url = `${CDN}/${t.slug}.${ext}`;
      let estado = 0;
      try {
        const r = await p.request.fetch(url, { headers: { range: 'bytes=0-1' }, timeout: 15000 });
        estado = r.status();
      } catch { /* queda en 0 y se reporta */ }
      /* 206 y no 200 porque el pedido lleva Range: el CDN sirve el trozo pedido. */
      check(`${t.slug}: el ${tipo} existe en el CDN`, estado === 200 || estado === 206, `${estado} en ${url}`);
    }
  }
  await p.context().close();
}

/* --------------------------------------------------------------------- run */
browser = await chromium.launch();
try {
  await testPages();
  await testLinks();
  await testFaq();
  await testCarousel();
  await testVideo();
  await testImages();
  await testResponsive();
  await testMobileMenu();
  await testComparisonTabs();
  await testAgents();
  await testContacto();
  await testRegistro();
  await testGuias();
  await testCostosMercadoPago();
  await testDatosMercadoPagoJson();
  await testNingunNumeroSuelto();
  await testEnlacesPegados();
  await testAnchoEnSerp();
  await testAssetsDeTutoriales();
} finally {
  await browser.close();
}

const failed = results.filter(r => !r.ok);
console.log(`\n${'='.repeat(58)}`);
console.log(`  ${results.length - failed.length}/${results.length} comprobaciones pasaron`);
if (failed.length) {
  console.log(`\n  FALLARON:`);
  failed.forEach(f => console.log(`    ✗ ${f.name}  ${f.detail}`));
}
process.exit(failed.length ? 1 : 0);
