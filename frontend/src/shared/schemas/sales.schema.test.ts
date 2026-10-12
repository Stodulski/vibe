import { describe, it, expect } from 'vitest';
// @vitest-environment node
import { makeBalanceDue, makeSale, makeSaleBookingPayment, makeSaleItem } from '@/test/factories';
import {
  saleSchema,
  saleItemSchema,
  saleStockWarningSchema,
  salesListResponseSchema,
  saleCreateResponseSchema,
  saleEnvelopeSchema,
  balancesDueResponseSchema,
} from './sales.schema';

describe('saleItemSchema', () => {
  it('validates a sale item', () => {
    expect(saleItemSchema.safeParse(makeSaleItem()).success).toBe(true);
  });
});

describe('saleSchema', () => {
  it('validates an active sale', () => {
    expect(saleSchema.safeParse(makeSale()).success).toBe(true);
  });

  it('validates a balance-only checkout entry: no items and no cash movement', () => {
    const entry = makeSale({
      items: [],
      cash_movement_id: null,
      total: 350000,
      booking_payments: [makeSaleBookingPayment({ amount: 350000 })],
    });
    expect(saleSchema.safeParse(entry).success).toBe(true);
  });

  it('validates a voided sale', () => {
    const sale = makeSale({ voided_at: '2026-01-02T10:00:00Z', voided_by: 'u1', void_cash_movement_id: 'cm2' });
    expect(saleSchema.safeParse(sale).success).toBe(true);
  });

  it('rejects an unknown method', () => {
    const sale = { ...makeSale(), method: 'crypto' };
    expect(saleSchema.safeParse(sale).success).toBe(false);
  });
});

describe('saleStockWarningSchema', () => {
  it('validates a stock warning', () => {
    const warning = { product_id: 'p1', product_name: 'Agua', stock_on_hand: -2 };
    expect(saleStockWarningSchema.safeParse(warning).success).toBe(true);
  });
});

describe('response envelopes', () => {
  it('validates a paginated sales list response', () => {
    const body = { sales: [makeSale()], metadata: { has_more: false } };
    expect(salesListResponseSchema.safeParse(body).success).toBe(true);
  });

  it('validates a sale-create response with stock warnings', () => {
    const body = {
      sale: makeSale(),
      stock_warnings: [{ product_id: 'p1', product_name: 'Agua', stock_on_hand: -1 }],
      booking_payments: [],
    };
    expect(saleCreateResponseSchema.safeParse(body).success).toBe(true);
  });

  it('validates a single-sale envelope', () => {
    expect(saleEnvelopeSchema.safeParse({ sale: makeSale() }).success).toBe(true);
  });

  it('validates a booking-only checkout, whose sale is null and whose balances are listed', () => {
    const body = {
      sale: null,
      stock_warnings: [],
      booking_payments: [makeSaleBookingPayment()],
    };
    expect(saleCreateResponseSchema.safeParse(body).success).toBe(true);
  });

  it('validates a sale that carries the balances collected with it', () => {
    const sale = makeSale({ booking_payments: [makeSaleBookingPayment()] });
    expect(saleSchema.safeParse(sale).success).toBe(true);
  });
});

describe('balancesDueResponseSchema', () => {
  it('validates the day list of bookings that still owe money', () => {
    const body = { date: '2026-10-12', bookings: [makeBalanceDue()] };
    expect(balancesDueResponseSchema.safeParse(body).success).toBe(true);
  });

  it('rejects a booking with an unknown status', () => {
    const body = { date: '2026-10-12', bookings: [{ ...makeBalanceDue(), status: 'crypto' }] };
    expect(balancesDueResponseSchema.safeParse(body).success).toBe(false);
  });
});
