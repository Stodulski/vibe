import { describe, it, expect } from 'vitest';
// @vitest-environment node
import { mpFeesResponseSchema } from './mpFees.schema';

const validResponse = {
  vigente_desde: '2026-03-06',
  fuente: 'https://www.mercadopago.com.ar/ayuda/33399',
  iva_incluido: false,
  plazos: ['Al instante', '10 días', '18 días', '35 días'],
  grupos: [
    { provincias: ['Buenos Aires', 'Chubut', 'Entre Ríos'], tasas: [6.6, 4.61, 3.56, 1.56] },
    { provincias: ['Ciudad de Buenos Aires'], tasas: [6.5, 4.5, 3.5, 1.5] },
  ],
  generado: '2026-09-08T20:00:00.000Z',
};

describe('mpFeesResponseSchema', () => {
  it('accepts the contract shape', () => {
    const result = mpFeesResponseSchema.safeParse(validResponse);
    expect(result.success).toBe(true);
  });

  it('rejects a response missing grupos', () => {
    const { grupos: _grupos, ...withoutGrupos } = validResponse;
    const result = mpFeesResponseSchema.safeParse(withoutGrupos);
    expect(result.success).toBe(false);
  });

  it('rejects a grupo whose tasas count does not match plazos', () => {
    const result = mpFeesResponseSchema.safeParse({
      ...validResponse,
      grupos: [{ provincias: ['Buenos Aires'], tasas: [6.6, 4.61] }],
    });
    expect(result.success).toBe(false);
  });

  it('rejects a non-URL fuente', () => {
    const result = mpFeesResponseSchema.safeParse({ ...validResponse, fuente: 'not-a-url' });
    expect(result.success).toBe(false);
  });
});
