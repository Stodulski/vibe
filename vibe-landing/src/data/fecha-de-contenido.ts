/**
 * La fecha en que el contenido de algo cambió por última vez, sin escribirla.
 *
 * Ya hay un mecanismo así en el repo: `page-dates.json`, que arma
 * scripts/build-markdown.mjs hasheando el HTML ya construido. Ese sirve para el
 * `lastmod` del sitemap, que se escribe DESPUÉS de renderizar.
 *
 * Este es otro porque el problema es otro. La fecha de una guía se muestra en la
 * página y viaja en su JSON-LD, así que hay que conocerla ANTES de renderizar.
 * Con el mecanismo post-build llegaría un deploy tarde: el build que trae el
 * cambio mostraría todavía la fecha vieja, y recién el siguiente la corregiría.
 * Una fecha que va atrás del contenido es peor que ninguna.
 *
 * Entonces se hashea el DATO, no la salida, y se resuelve en el mismo build.
 *
 * Por qué no `git log -- <archivo>`: Vercel construye desde un clon superficial,
 * donde todo archivo reporta el commit de cabeza haya cambiado o no. El commit
 * de cabeza sí se puede leer (`git log -1`), y es de ahí que sale la fecha
 * cuando el contenido efectivamente cambió.
 *
 * Build-time only: usa fs y git, nunca corre en el navegador.
 */
import { createHash } from 'node:crypto';
import { readFileSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { LAST_MODIFIED } from './site-dates.ts';

/* La ruta se arma desde cwd y no desde `import.meta.url` porque Vite reescribe
   ese valor al empaquetar, y durante el build de Astro el write se iba a un lado
   que no es este: el mensaje salia en consola y el archivo quedaba sin tocar.
   Astro y el buildCommand de Vercel corren los dos desde la raiz del proyecto,
   igual que el `git log` de site-dates.ts. */
const ARCHIVO = join(process.cwd(), 'src', 'data', 'guias-fechas.json');

type Registro = Record<string, { hash: string; date: string }>;

function leer(): Registro {
  try { return JSON.parse(readFileSync(ARCHIVO, 'utf8')); } catch { return {}; }
}

const registro = leer();
let cambio = false;

/** Huella estable de un objeto, con las claves ordenadas. */
function huella(contenido: unknown): string {
  const estable = JSON.stringify(contenido, (_, v) =>
    v && typeof v === 'object' && !Array.isArray(v)
      ? Object.fromEntries(Object.entries(v).sort(([a], [b]) => a.localeCompare(b)))
      : v);
  return createHash('sha256').update(estable ?? '').digest('hex').slice(0, 16);
}

/**
 * La fecha de `clave`. Se queda con la guardada mientras el contenido no cambie,
 * y toma la del commit actual cuando efectivamente cambia.
 */
export function fechaDeContenido(clave: string, contenido: unknown): string {
  const h = huella(contenido);
  const previo = registro[clave];
  if (!previo || previo.hash !== h) {
    registro[clave] = { hash: h, date: LAST_MODIFIED };
    cambio = true;
  }
  return registro[clave].date;
}

/**
 * Persiste lo que haya cambiado. Se llama una vez, después de resolver todas las
 * fechas, porque escribir por cada una haría un write por guía sin necesidad.
 *
 * El archivo se commitea con el cambio, igual que page-dates.json: si no, cada
 * build en limpio creería que todo es nuevo y le pondría la fecha de hoy a todo.
 */
export function guardarFechas(): void {
  if (!cambio) return;
  writeFileSync(ARCHIVO, JSON.stringify(registro, null, 2) + '\n', 'utf8');
  console.log('  src/data/guias-fechas.json actualizado — commitealo con el cambio');
}
