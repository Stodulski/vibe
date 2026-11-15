import { describe, it, expect } from 'vitest';
import { ES_AR } from './es_AR';

describe('ES_AR', () => {
  it('spells the payment provider "Mercado Pago", never "MercadoPago"', () => {
    expect(JSON.stringify(ES_AR)).not.toContain('MercadoPago');
  });
});
