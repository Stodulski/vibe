import { cleanBookingPayload } from './cleanBookingPayload';
import type { CreateBookingDto } from '../../schemas/booking.schema';

const base: CreateBookingDto = {
  court_id: 'ct1',
  date: '2099-01-01',
  start_time: '10:00',
  duration_minutes: 90,
  client_phone: '+541155550000',
  client_first_name: 'Juan',
  client_last_name: 'Perez',
};

describe('cleanBookingPayload', () => {
  it('converts an empty notes string to undefined', () => {
    expect(cleanBookingPayload({ ...base, notes: '' }).notes).toBeUndefined();
  });

  it('preserves a non-empty notes string', () => {
    expect(cleanBookingPayload({ ...base, notes: 'Traer paletas' }).notes).toBe('Traer paletas');
  });

  it('defaults payment_option to "unpaid" when empty', () => {
    expect(cleanBookingPayload({ ...base, payment_option: undefined }).payment_option).toBe('unpaid');
  });

  it('converts deposit_amount from pesos to cents only for the "deposit" option', () => {
    const result = cleanBookingPayload({
      ...base,
      payment_option: 'deposit',
      payment_method: 'cash',
      deposit_amount: 150,
    });
    expect(result.deposit_amount).toBe(15000);
  });

  it('drops deposit_amount for non-deposit payment options', () => {
    const result = cleanBookingPayload({
      ...base,
      payment_option: 'full',
      payment_method: 'cash',
      deposit_amount: 150,
    });
    expect(result.deposit_amount).toBeUndefined();
  });

  it('passes duration_minutes through unchanged', () => {
    expect(cleanBookingPayload({ ...base, duration_minutes: 120 }).duration_minutes).toBe(120);
  });
});
