import type { BalanceDue, SaleBookingPayment } from '@/shared/types/api.types';

// Booking-balance fixtures for the sell page's "Turnos de hoy" and the sales
// rows that carry them. Re-exported from `factories.ts`, so tests keep one
// import path.

const defaultSaleBookingPayment: SaleBookingPayment = {
  payment_id: 'pay1',
  booking_id: 'b1',
  amount: 350000,
  court_name: 'Cancha 2',
  date: '2026-10-12',
  start_time: '19:00',
  end_time: '20:30',
  client_name: 'Juan Pérez',
};

/** Builds a typed `SaleBookingPayment` fixture with sensible defaults, overridable per-field. */
export function makeSaleBookingPayment(overrides: Partial<SaleBookingPayment> = {}): SaleBookingPayment {
  return { ...defaultSaleBookingPayment, ...overrides };
}

const defaultBalanceDue: BalanceDue = {
  booking_id: 'b1',
  court_name: 'Cancha 2',
  date: '2026-10-12',
  start_time: '19:00',
  end_time: '20:30',
  client_name: 'Juan Pérez',
  status: 'confirmed',
  total: 500000,
  paid: 150000,
  remaining: 350000,
};

/** Builds a typed `BalanceDue` fixture (one booking of today that still owes money), overridable per-field. */
export function makeBalanceDue(overrides: Partial<BalanceDue> = {}): BalanceDue {
  return { ...defaultBalanceDue, ...overrides };
}
