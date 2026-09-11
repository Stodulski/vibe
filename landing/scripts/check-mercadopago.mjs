/**
 * Vigila que src/data/mercadopago-costos.ts siga diciendo lo que dice MercadoPago.
 *
 *   node scripts/check-mercadopago.mjs            detecta, no escribe nada
 *   node scripts/check-mercadopago.mjs --write     detecta y, si hay diferencias
 *                                                   de valores, reescribe el dataset
 *
 * Sin --write la diferencia no es pereza. Los porcentajes de esta tabla son una
 * afirmación sobre plata en la página de precios. Que cambien solos porque un
 * parser interpretó distinto un HTML que rediseñaron es peor que que queden
 * viejos: viejo se nota, mal no. Por eso --write nunca toca el archivo si el
 * guard de parseo no pasa, aunque se lo pidan explícitamente: escribir sobre un
 * parseo que no se pudo confirmar sería automatizar justamente el riesgo que
 * el guard existe para frenar.
 *
 * Códigos de salida:
 *   0   sin diferencias (o, con --write, diferencias que ya se escribieron)
 *   1   los VALORES difieren de lo publicado (parseo confiable, dataset viejo)
 *   2   el parseo no es confiable (guard de estructura, sin fecha, o no se
 *       pudo leer la fuente) — esto necesita ojos humanos, nunca un --write
 *
 * Sobre el parseo, que es la parte frágil y conviene tener presente:
 *
 * - MercadoPago responde 403 a cualquier cliente que no parezca un navegador,
 *   así que va con user agent de Chrome. No hace falta navegador de verdad: los
 *   porcentajes vienen en el HTML de la respuesta.
 * - El contenido viaja como HTML doblemente escapado adentro de un blob JS. Hay
 *   que resolver primero los `\uXXXX`, después los `\n`, y recién ahí sacar las
 *   barras que sobran. En otro orden los saltos de línea quedan como una "n"
 *   pegada entre etiquetas y no matchea nada.
 * - EL GUARD DE ABAJO NO ES OPCIONAL. Una versión anterior de este parseo traía
 *   5 de los 9 grupos sin quejarse. Un chequeo que aprueba con datos parciales es
 *   peor que no tenerlo, porque da por confirmado algo que no miró.
 */
import { readFile, writeFile } from 'node:fs/promises';
import { COSTOS, PLAZOS, VIGENTE_DESDE, FUENTE, porcentaje } from '../src/data/mercadopago-costos.ts';

const WRITE = process.argv.includes('--write');
const DATA_FILE = new URL('../src/data/mercadopago-costos.ts', import.meta.url);

const UA = 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 '
  + '(KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36';

const MESES = [
  'enero', 'febrero', 'marzo', 'abril', 'mayo', 'junio',
  'julio', 'agosto', 'septiembre', 'octubre', 'noviembre', 'diciembre',
];

const problemas = [];
const fallar = (msg) => problemas.push(msg);

/** El HTML util, sacado del blob JS. El orden de los reemplazos importa. */
function desescapar(raw) {
  return raw
    .replace(/\\u([0-9a-fA-F]{4})/g, (_, h) => String.fromCharCode(parseInt(h, 16)))
    .replace(/\\+n/g, '\n')
    .replace(/\\+/g, '');
}

const limpiar = (s) => s.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim();

/** "6 de marzo de 2026" -> "2026-03-06". */
function fechaISO(texto) {
  const m = /(\d{1,2}) de ([a-záéíóú]+) de (\d{4})/i.exec(texto);
  if (!m) return null;
  const mes = MESES.indexOf(m[2].toLowerCase());
  if (mes < 0) return null;
  return `${m[3]}-${String(mes + 1).padStart(2, '0')}-${m[1].padStart(2, '0')}`;
}

/* Cada grupo es un <p> con las provincias en <strong> y su <table> abajo. */
function extraerGrupos(doc) {
  const re = /<p[^>]*>\s*<strong>([^<]*?):\s*<\/strong>\s*<\/p>\s*(<table[\s\S]*?<\/table>)/g;
  const grupos = [];
  for (const m of doc.matchAll(re)) {
    grupos.push({
      provincias: limpiar(m[1]).split('|').map(p => p.trim()).filter(Boolean),
      tasas: [...limpiar(m[2]).matchAll(/(\d+,\d+)\s*%/g)].map(x => Number(x[1].replace(',', '.'))),
    });
  }
  return grupos;
}

const clave = (provincias) => [...provincias].sort().join(' | ');

/**
 * Cierra el programa por un guard de parseo (no un valor distinto): grupos de
 * más o de menos, tasas incompletas, o ninguna fecha de vigencia en la página.
 * En cualquiera de los tres casos no hay nada confiable para comparar, y con
 * --write tampoco hay nada confiable para escribir.
 */
function guardFail(lineas) {
  lineas.forEach(l => console.error(l));
  if (WRITE) {
    console.error('\n--write no escribe nada: el parseo no pasó el guard, así que no hay valores');
    console.error('confiables para volcar al dataset. src/data/mercadopago-costos.ts queda intacto.');
  }
  process.exit(2);
}

/* ---------------------------------------------------------- formato del .ts */

/* Reproduce a mano el estilo con el que está escrito el archivo (no pasó por
 * un formateador: los números conservan sus dos decimales y las flechas no
 * llevan paréntesis, cosas que Prettier deshace). Los anchos están pisados
 * contra el archivo actual línea por línea; si algún día se lo reformatea con
 * una herramienta, esto se puede borrar entero. */
const ENTRY_WIDTH = 100;
const PROVINCIAS_LINE_WIDTH = 100;
const FILL_WIDTH = 80;

/** Empaqueta provincias de a varias por línea, tal como quedó a mano el grupo de 9. */
function fillItems(items, indent, width) {
  const lineas = [];
  let cur = indent;
  let cuenta = 0;
  for (let i = 0; i < items.length; i++) {
    const esUltimo = i === items.length - 1;
    const token = `'${items[i]}',`;
    const sep = esUltimo ? '' : ' ';
    const largoCandidato = cur.length + token.length + sep.length;
    if (cuenta > 0 && largoCandidato > width) {
      lineas.push(cur.replace(/\s+$/, ''));
      cur = indent;
      cuenta = 0;
    }
    cur += token + (esUltimo ? '' : sep);
    cuenta++;
  }
  if (cuenta > 0) lineas.push(cur);
  return lineas;
}

function formatGrupo(g, indent = '  ') {
  const tasasStr = g.tasas.map(t => t.toFixed(2)).join(', ');
  const provinciasInline = g.provincias.map(p => `'${p}'`).join(', ');
  const unaLinea = `${indent}{ provincias: [${provinciasInline}], tasas: [${tasasStr}] },`;
  if (unaLinea.length <= ENTRY_WIDTH) return unaLinea;

  const provinciasLinea = `${indent}  provincias: [${provinciasInline}],`;
  const provinciasBlock = provinciasLinea.length <= PROVINCIAS_LINE_WIDTH
    ? provinciasLinea
    : `${indent}  provincias: [\n${fillItems(g.provincias, `${indent}    `, FILL_WIDTH).join('\n')}\n${indent}  ],`;
  return `${indent}{\n${provinciasBlock}\n${indent}  tasas: [${tasasStr}],\n${indent}},`;
}

/**
 * Reescribe src/data/mercadopago-costos.ts, tocando solo VIGENTE_DESDE y el
 * array COSTOS. Todo lo demás del archivo (comentarios, tipos, funciones
 * derivadas) queda exactamente como estaba.
 */
async function escribirDataset(nuevaVigencia, nuevosGrupos) {
  const original = await readFile(DATA_FILE, 'utf8');

  const conVigencia = original.replace(
    /export const VIGENTE_DESDE = '[^']*';/,
    `export const VIGENTE_DESDE = '${nuevaVigencia}';`,
  );
  if (conVigencia === original) {
    throw new Error('No se encontró la línea de VIGENTE_DESDE para reemplazar.');
  }

  const nuevoBlock = `export const COSTOS: GrupoDeCostos[] = [\n`
    + `${nuevosGrupos.map(g => formatGrupo(g)).join('\n')}\n];`;
  const conCostos = conVigencia.replace(
    /export const COSTOS: GrupoDeCostos\[\] = \[[\s\S]*?\n\];/,
    nuevoBlock,
  );
  if (conCostos === conVigencia) {
    throw new Error('No se encontró el array COSTOS para reemplazar.');
  }

  await writeFile(DATA_FILE, conCostos, 'utf8');
}

/* ------------------------------------------------------------------ main */

let res;
try {
  res = await fetch(FUENTE, { headers: { 'user-agent': UA, 'accept-language': 'es-AR,es;q=0.9' } });
} catch (e) {
  guardFail([`No se pudo conectar a ${FUENTE}: ${e.message}`]);
}
if (!res.ok) {
  guardFail([
    `No se pudo leer ${FUENTE}: HTTP ${res.status}`,
    'Si es 403, MercadoPago cambió su bloqueo de bots y hay que revisar el user agent.',
  ]);
}

const doc = desescapar(await res.text());
const grupos = extraerGrupos(doc);

/* --- El guard. Antes de comparar (y antes de escribir) nada, probar que se leyó todo. --- */
if (grupos.length !== COSTOS.length) {
  guardFail([
    `El parseo trajo ${grupos.length} grupos y el dataset tiene ${COSTOS.length}.`,
    'Puede ser que MercadoPago agregó o sacó un grupo, o que cambió el markup y',
    'este script quedó viejo. En los dos casos hay que mirarlo a mano antes de',
    'confiar en cualquier comparación. No se compara nada más.',
    ...grupos.map(g => `  leído: ${g.provincias.join(' | ')} -> ${g.tasas.join(' ')}`),
  ]);
}

const incompletos = grupos.filter(g => g.tasas.length !== PLAZOS.length);
if (incompletos.length) {
  guardFail([
    `Hay ${incompletos.length} grupo(s) sin sus ${PLAZOS.length} tasas. El parseo no es confiable.`,
    ...incompletos.map(g => `  ${g.provincias.join(' | ')} -> ${g.tasas.join(' ')}`),
  ]);
}

const vigencia = fechaISO(doc.slice(doc.indexOf('Costos vigentes a partir del')));
if (!vigencia) {
  guardFail(['No se encontró la fecha de vigencia en la página. Revisar el markup.']);
}

/* --- Recién ahora, con el guard pasado, comparar valores. --- */
if (vigencia !== VIGENTE_DESDE) {
  fallar(`La vigencia cambió: el dataset dice ${VIGENTE_DESDE} y la página dice ${vigencia}.`);
}

const publicados = new Map(grupos.map(g => [clave(g.provincias), g]));

for (const nuestro of COSTOS) {
  const k = clave(nuestro.provincias);
  const suyo = publicados.get(k);
  if (!suyo) {
    fallar(`El grupo "${nuestro.provincias.join(', ')}" ya no figura con esas provincias.`);
    continue;
  }
  nuestro.tasas.forEach((tasa, i) => {
    if (Math.abs(tasa - suyo.tasas[i]) > 0.001) {
      fallar(`${nuestro.provincias[0]} / ${PLAZOS[i]}: `
        + `el dataset dice ${porcentaje(tasa)} y MercadoPago publica ${porcentaje(suyo.tasas[i])}.`);
    }
  });
}

for (const g of grupos) {
  if (!COSTOS.some(n => clave(n.provincias) === clave(g.provincias))) {
    fallar(`Grupo nuevo en MercadoPago que el dataset no tiene: ${g.provincias.join(', ')}.`);
  }
}

if (problemas.length) {
  console.error(`Los costos de MercadoPago cambiaron (${problemas.length} diferencia(s)):\n`);
  problemas.forEach(p => console.error(`  - ${p}`));
  console.error(`\nFuente: ${FUENTE}`);

  if (!WRITE) {
    console.error('\nQué hacer: confirmar en la fuente, actualizar src/data/mercadopago-costos.ts');
    console.error('y mover VIGENTE_DESDE en el mismo commit. La suite de tests verifica que la');
    console.error('home y la guía muestren el dataset, así que las dos se actualizan solas.');
    process.exit(1);
  }

  await escribirDataset(vigencia, grupos);
  console.log('\nsrc/data/mercadopago-costos.ts reescrito: VIGENTE_DESDE y COSTOS ahora dicen lo');
  console.log('mismo que la fuente. La suite de tests verifica que la home y la guía muestren');
  console.log('el dataset, así que las dos quedan al día solas.');
  process.exit(0);
}

console.log(`Los ${grupos.length} grupos coinciden con el dataset. Vigencia ${vigencia}.`);
