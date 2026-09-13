import { z } from 'zod';

// Published by the landing (`VITE_LANDING_URL` + the fixed `/datos/mercadopago-costos.json`
// path), not by vibe's own API — there is no corresponding hand-written type in
// `src/shared/types/api.types`, so this schema is its own source of truth via `z.infer`
// rather than going through `exact<T>`.

const mpFeesGroupSchema = z
  .object({
    provincias: z.array(z.string()),
    tasas: z.array(z.number()),
  })
  .loose();

export const mpFeesResponseSchema = z
  .object({
    vigente_desde: z.string(),
    fuente: z.url(),
    iva_incluido: z.boolean(),
    plazos: z.array(z.string()),
    grupos: z.array(mpFeesGroupSchema),
    generado: z.string(),
  })
  .loose()
  // `tasas[i]` is read against `plazos[i]` by position (see `MPFeesSection`) —
  // a group with a different rate count than there are plazos would silently
  // misalign every row after the first mismatch instead of failing to parse.
  .refine((data) => data.grupos.every((grupo) => grupo.tasas.length === data.plazos.length), {
    message: 'each grupo.tasas must have exactly one rate per plazo',
    path: ['grupos'],
  });

export type MPFeesGroup = z.infer<typeof mpFeesGroupSchema>;
export type MPFeesResponse = z.infer<typeof mpFeesResponseSchema>;
