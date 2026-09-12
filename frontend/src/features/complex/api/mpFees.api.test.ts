import { describe, it, expect, vi } from 'vitest';
// @vitest-environment node
import { fetchMPFees } from './mpFees.api';
import { ApiResponseError } from '@/shared/lib/apiParse';

const validResponse = {
  vigente_desde: '2026-03-06',
  fuente: 'https://www.mercadopago.com.ar/ayuda/33399',
  iva_incluido: false,
  plazos: ['Al instante', '10 días', '18 días', '35 días'],
  grupos: [{ provincias: ['Buenos Aires'], tasas: [6.6, 4.61, 3.56, 1.56] }],
  generado: '2026-09-08T20:00:00.000Z',
};

describe('fetchMPFees', () => {
  it('fetches and parses the landing JSON contract from VITE_LANDING_URL', async () => {
    const mockFetch = vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(validResponse) });
    vi.stubGlobal('fetch', mockFetch);

    const result = await fetchMPFees();

    expect(result).toEqual(validResponse);
    const [url] = mockFetch.mock.calls[0] as [URL];
    expect(url.toString()).toBe('https://vibe.com.ar/datos/mercadopago-costos.json');

    vi.unstubAllGlobals();
  });

  it('throws on a non-ok response', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, status: 404 }));

    await expect(fetchMPFees()).rejects.toThrow();

    vi.unstubAllGlobals();
  });

  it('throws ApiResponseError on a malformed body', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve({}) }));

    await expect(fetchMPFees()).rejects.toBeInstanceOf(ApiResponseError);

    vi.unstubAllGlobals();
  });
});
