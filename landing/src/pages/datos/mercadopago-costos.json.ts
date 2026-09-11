/**
 * Espejo en JSON de src/data/mercadopago-costos.ts, para que la app (otro
 * origen, app.vibe.com.ar) pueda leer los mismos números sin repetirlos a
 * mano en otro repositorio.
 *
 * Se genera en build, no a mano: el dataset sigue siendo el único lugar que
 * se edita, acá solo se lo serializa. `generado` es el momento del build, no
 * un dato del dataset, así que un consumidor puede saber qué tan fresco es
 * este archivo sin depender de los headers de caché.
 */
import type { APIRoute } from 'astro';
import { VIGENTE_DESDE, FUENTE, PLAZOS, COSTOS } from '../../data/mercadopago-costos.ts';

export const prerender = true;

export const GET: APIRoute = () => {
  const body = {
    vigente_desde: VIGENTE_DESDE,
    fuente: FUENTE,
    iva_incluido: false,
    plazos: [...PLAZOS],
    grupos: COSTOS.map(g => ({ provincias: g.provincias, tasas: g.tasas })),
    generado: new Date().toISOString(),
  };

  return new Response(JSON.stringify(body, null, 2), {
    headers: { 'Content-Type': 'application/json' },
  });
};
