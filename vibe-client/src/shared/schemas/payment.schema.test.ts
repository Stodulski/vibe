// @vitest-environment node
import { paymentSchema } from './payment.schema';

const validPayment = {
  id: 'pay1',
  booking_id: 'b1',
  complex_id: 'c1',
  amount: 5000,
  service_fee: 250,
  method: 'mercadopago',
  status: 'approved',
  refund_amount: 0,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
};

describe('paymentSchema', () => {
  it('validates a realistic Payment fixture', () => {
    expect(paymentSchema.safeParse(validPayment).success).toBe(true);
  });

  it('accepts an optional mp_payment_id', () => {
    const result = paymentSchema.safeParse({ ...validPayment, mp_payment_id: 'mp1' });
    expect(result.success).toBe(true);
  });

  it('rejects a payment missing amount', () => {
    const { amount: _amount, ...withoutAmount } = validPayment;
    expect(paymentSchema.safeParse(withoutAmount).success).toBe(false);
  });
});
