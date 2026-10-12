import { describe, it, expect } from 'vitest';
import { makeBalanceDue } from '@/test/factories';
import {
  MAX_BALANCE_LINES,
  addBalanceLine,
  balancesTotal,
  bookingBalanceLabel,
  reconcileBalances,
  removeBalanceLine,
  type BalanceLine,
} from './balanceCart';

const line = (bookingId: string, amount = 100000): BalanceLine => ({
  bookingId,
  courtName: 'Cancha 1',
  startTime: '10:00',
  clientName: 'Cliente',
  amount,
});

describe('addBalanceLine', () => {
  it('adds the booking with its remaining balance as the amount, labels included', () => {
    const booking = makeBalanceDue({
      booking_id: 'b1',
      court_name: 'Cancha 2',
      start_time: '19:00',
      client_name: 'Juan Pérez',
      remaining: 350000,
    });

    expect(addBalanceLine([], booking)).toEqual([
      { bookingId: 'b1', courtName: 'Cancha 2', startTime: '19:00', clientName: 'Juan Pérez', amount: 350000 },
    ]);
  });

  it('never adds the same booking twice', () => {
    const booking = makeBalanceDue({ booking_id: 'b1' });
    const once = addBalanceLine([], booking);

    expect(addBalanceLine(once, booking)).toBe(once);
  });

  it('refuses a booking past the cap the server accepts (20 per checkout)', () => {
    const full = Array.from({ length: MAX_BALANCE_LINES }, (_, i) => line(`b${String(i)}`));

    expect(addBalanceLine(full, makeBalanceDue({ booking_id: 'extra' }))).toBe(full);
    expect(MAX_BALANCE_LINES).toBe(20);
  });
});

describe('removeBalanceLine', () => {
  it('removes the line for that booking and keeps the others', () => {
    expect(removeBalanceLine([line('b1'), line('b2')], 'b1')).toEqual([line('b2')]);
  });
});

describe('balancesTotal', () => {
  it('sums the amounts, and is zero for an empty cart', () => {
    expect(balancesTotal([line('b1', 350000), line('b2', 200000)])).toBe(550000);
    expect(balancesTotal([])).toBe(0);
  });
});

describe('reconcileBalances', () => {
  it('keeps a line whose booking still owes exactly the amount in the cart', () => {
    const booking = makeBalanceDue({ booking_id: 'b1', remaining: 100000 });

    expect(reconcileBalances([line('b1', 100000)], [booking])).toEqual({ lines: [line('b1', 100000)], dropped: false });
  });

  it('drops a line whose booking left the day list (paid, cancelled, no-show or out of its window)', () => {
    const result = reconcileBalances(
      [line('b1'), line('b2')],
      [makeBalanceDue({ booking_id: 'b2', remaining: 100000 })],
    );

    expect(result.lines.map((l) => l.bookingId)).toEqual(['b2']);
    expect(result.dropped).toBe(true);
  });

  it('drops a line whose remaining balance changed, so the cashier sees the new amount first', () => {
    const booking = makeBalanceDue({ booking_id: 'b1', remaining: 80000 });

    const result = reconcileBalances([line('b1', 100000)], [booking]);

    expect(result.lines).toEqual([]);
    expect(result.dropped).toBe(true);
  });
});

describe('bookingBalanceLabel', () => {
  it('reads as the cart line the cashier taps: prefix, court and start time, then the client', () => {
    expect(bookingBalanceLabel('Cancha 2', '19:00', 'Juan Pérez')).toBe('Saldo turno · Cancha 2 19:00 · Juan Pérez');
  });
});
