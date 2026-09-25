/**
 * La portada que se ve cuando alguien comparte un enlace de vibe.com.ar.
 *
 * Se genera en vez de dibujarse a mano por el mismo motivo que el resto de lo
 * derivado en este repo: el titular tiene que decir lo mismo que el h1 del home,
 * y dos copias del mismo texto terminan siempre con una desactualizada. Acá el
 * texto está una vez, y el PNG sale de renderizar el HTML con la misma fuente y
 * los mismos colores que el sitio.
 *
 * 1200x630 es la medida que piden WhatsApp, Facebook, LinkedIn y Twitter para
 * la tarjeta grande. Cada uno recorta distinto, así que nada importante entra
 * en los 80px del borde.
 *
 * Uso: node scripts/make-og-image.mjs   (escribe public/og-image.png)
 */
import { chromium } from 'playwright';
import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

const RAIZ = process.cwd();
const LOGO = readFileSync(join(RAIZ, 'public', 'logo.svg'), 'utf8');
const SALIDA = join(RAIZ, 'public', 'og-image.png');

/*
 * El mismo eyebrow y el mismo h1 del home (landing-integral-positioning,
 * deck v2 §2.3), con el mismo quiebre de línea: la categoría arriba
 * ("sistema de gestión", no "reservas online"), después la promesa central
 * de la reescritura (orden + 100% gratis), en vez del viejo "Digitalización
 * de complejos deportivos" que todavía encuadraba el producto como reservas.
 */
const EYEBROW = 'Sistema de gestión para complejos deportivos';
const TITULAR = 'Tu complejo en orden.';
const RESALTADO = '100% gratis para vos.';

/**
 * Los mismos iconos que orbitan en el hero, quietos y del lado que el texto
 * deja libre. Tamaños y posiciones sin simetría, como en el hero: una grilla
 * pareja los haría leer como una lista de deportes soportados en vez de como
 * fondo.
 *
 * Van en base64 y no por ruta: setContent renderiza sin URL base, así que un
 * src relativo no resuelve y el icono saldría roto sin avisar.
 */
/*
 * Tamaños ~1.75x los originales (mismo fold que .sport-emoji en el hero,
 * pedido del dueño 2026-09-25 porque los iconos se veian chicos). El de
 * tennis ademas se corrio de x:1010 a x:980 porque agrandado se salia del
 * lienzo de 1200px de ancho.
 */
const ICONOS = [
  { archivo: 'soccer-ball.webp', x: 742, y: 96, tam: 263, giro: -14, opacidad: 1 },
  { archivo: 'tennis.webp', x: 980, y: 268, tam: 207, giro: 12, opacidad: 0.95 },
  { archivo: 'basketball.webp', x: 868, y: 398, tam: 168, giro: -8, opacidad: 0.9 },
  { archivo: 'volleyball.webp', x: 1046, y: 62, tam: 130, giro: 18, opacidad: 0.8 },
  { archivo: 'field-hockey.webp', x: 726, y: 492, tam: 112, giro: -20, opacidad: 0.7 },
  { archivo: 'trophy.webp', x: 1054, y: 462, tam: 102, giro: 8, opacidad: 0.62 },
];

const dataUri = (archivo) =>
  `data:image/webp;base64,${readFileSync(join(RAIZ, 'public', 'emoji', archivo)).toString('base64')}`;

const iconosHtml = ICONOS.map(
  (i) =>
    `<img class="deporte" src="${dataUri(i.archivo)}" alt=""
      style="left:${i.x}px;top:${i.y}px;width:${i.tam}px;height:${i.tam}px;
             transform:rotate(${i.giro}deg);opacity:${i.opacidad}">`
).join('\n  ');

const html = `<!doctype html>
<html lang="es"><head><meta charset="utf-8">
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=DM+Sans:ital,opsz,wght@0,9..40,400;0,9..40,700;1,9..40,700&display=swap" rel="stylesheet">
<style>
  *{margin:0;padding:0;box-sizing:border-box}
  body{width:1200px;height:630px;background:#0b0b0b;font-family:'DM Sans',system-ui,sans-serif;
       color:#fff;overflow:hidden;position:relative}
  /* El mismo resplandor verde que el fondo del sitio, apoyado detrás del logo. */
  .glow{position:absolute;width:900px;height:900px;left:-220px;top:-380px;border-radius:50%;
        background:radial-gradient(circle,rgba(29,185,84,.22) 0%,rgba(29,185,84,0) 62%)}
  .glow.b{left:auto;right:-300px;top:auto;bottom:-460px;width:820px;height:820px;
          background:radial-gradient(circle,rgba(29,185,84,.14) 0%,rgba(29,185,84,0) 62%)}
  .caja{position:relative;height:100%;padding:80px;display:flex;flex-direction:column;
        justify-content:center;gap:34px}
  .logo svg{width:92px;height:92px;display:block}
  .eyebrow{font-size:24px;font-weight:600;letter-spacing:.01em;color:rgba(255,255,255,.78);
           text-transform:uppercase}
  h1{font-size:72px;line-height:1.08;font-weight:700;letter-spacing:-.02em;max-width:930px}
  h1 em{font-style:italic;color:#1DB954;display:block}
  .pie{position:absolute;left:80px;bottom:76px;display:flex;align-items:center;gap:14px;
       font-size:26px;color:rgba(255,255,255,.62)}
  .punto{width:9px;height:9px;border-radius:50%;background:#1DB954}
  /* El mismo halo verde que .sport-emoji en el sitio. */
  .deporte{position:absolute;z-index:0;
    filter:drop-shadow(0 0 1px #1DB954) drop-shadow(0 0 1px #1DB954)
           drop-shadow(0 0 18px rgba(29,185,84,.55))}
  .caja,.pie{z-index:1}
</style></head>
<body>
  <div class="glow"></div><div class="glow b"></div>
  ${iconosHtml}
  <div class="caja">
    <div class="logo">${LOGO}</div>
    <p class="eyebrow">${EYEBROW}</p>
    <h1>${TITULAR}<em>${RESALTADO}</em></h1>
  </div>
  <div class="pie"><span class="punto"></span>vibe.com.ar</div>
</body></html>`;

const navegador = await chromium.launch();
const pagina = await navegador.newPage({ viewport: { width: 1200, height: 630 }, deviceScaleFactor: 1 });
await pagina.setContent(html, { waitUntil: 'networkidle' });
/* Sin esto el screenshot sale a veces con la fuente de respaldo. */
await pagina.evaluate(() => document.fonts.ready);
writeFileSync(SALIDA, await pagina.screenshot({ type: 'png' }));
await navegador.close();

console.log(`escrito ${SALIDA} (1200x630)`);
