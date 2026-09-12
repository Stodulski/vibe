// @vitest-environment node
import { paymentSchema } from './payment.schema';

const validPayment = {
  id: 'pay1',
  booking_id: 'b1',
  complex_id: 'c1',
  amount: 5000,
  service_fee: 250,
  method: 'mercadopago',
  status: 'fully_paid',
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

  it('keeps a stored row whose method or status predates this build', () => {
    // A payments row is history: the server may have written a member that
    // has since been renamed or retired, and failing the parse would blank
    // the whole booking-detail response over a label.
    const legacy = paymentSchema.safeParse({ ...validPayment, method: 'bank_debit', status: 'approved' });
    expect(legacy.success).toBe(true);
    expect(legacy.success && legacy.data.status).toBe('approved');
  });

  it('accepts a null mp_payment_id, mp_preference_id and status_detail', () => {
    const result = paymentSchema.safeParse({
      ...validPayment,
      mp_payment_id: null,
      mp_preference_id: null,
      status_detail: null,
    });
    expect(result.success).toBe(true);
  });

  it('rejects a payment missing amount', () => {
    const { amount: _amount, ...withoutAmount } = validPayment;
    expect(paymentSchema.safeParse(withoutAmount).success).toBe(false);
  });
});
