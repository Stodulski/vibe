import { describe, it, expect } from 'vitest';
import { COUNTER_PAYMENT_METHODS } from './paymentMethods';

describe('COUNTER_PAYMENT_METHODS', () => {
  it('lists every method but mercadopago', () => {
    expect(COUNTER_PAYMENT_METHODS).toEqual(['cash', 'transfer', 'debit_card', 'credit_card', 'qr_wallet']);
    expect(COUNTER_PAYMENT_METHODS).not.toContain('mercadopago');
  });
});
