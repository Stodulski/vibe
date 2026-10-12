import { describe, it, expect, beforeEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { makeBalanceDue } from '@/test/factories';
import { saveBalanceCart } from '../../lib/cartStorage';
import type { BalanceLine } from '../../lib/balanceCart';
import { useSellBalances, type DueSnapshot } from './useSellBalances';

const COMPLEX = 'complex-balances';
const cashier = makeBalanceDue({ booking_id: 'b1', court_name: 'Cancha 2', start_time: '19:00', remaining: 350000 });
const second = makeBalanceDue({
  booking_id: 'b2',
  court_name: 'Cancha 1',
  start_time: '20:00',
  client_name: 'Ana Gómez',
  remaining: 200000,
});

const storedLine: BalanceLine = {
  bookingId: 'b1',
  courtName: 'Cancha 2',
  startTime: '19:00',
  clientName: 'Juan Pérez',
  amount: 350000,
};

const snapshot = (bookings: ReturnType<typeof makeBalanceDue>[], stamp = 1): DueSnapshot => ({ bookings, stamp });

beforeEach(() => {
  sessionStorage.clear();
});

describe('useSellBalances — restoring the cart', () => {
  it('waits for the day list before it restores the stored balances', () => {
    saveBalanceCart(COMPLEX, [storedLine]);
    const { result, rerender } = renderHook(({ due }) => useSellBalances(COMPLEX, due), {
      initialProps: { due: null as DueSnapshot | null },
    });

    expect(result.current.lines).toEqual([]);

    rerender({ due: snapshot([cashier]) });

    expect(result.current.lines).toEqual([storedLine]);
    expect(result.current.droppedNotice).toBe(false);
  });

  it('drops a restored line whose booking no longer owes that amount, and says so once', () => {
    saveBalanceCart(COMPLEX, [storedLine]);
    const { result, rerender } = renderHook(({ due }) => useSellBalances(COMPLEX, due), {
      initialProps: { due: null as DueSnapshot | null },
    });

    rerender({ due: snapshot([makeBalanceDue({ booking_id: 'b1', remaining: 100000 })]) });

    expect(result.current.lines).toEqual([]);
    expect(result.current.droppedNotice).toBe(true);

    act(() => {
      result.current.dismissDroppedNotice();
    });
    expect(result.current.droppedNotice).toBe(false);
  });
});

describe('useSellBalances — adding and removing', () => {
  it('adds a booking once, tracks the total, and removes it', () => {
    const { result, rerender } = renderHook(({ due }) => useSellBalances(COMPLEX, due), {
      initialProps: { due: snapshot([cashier, second]) },
    });

    act(() => {
      result.current.add(cashier);
    });
    act(() => {
      result.current.add(cashier);
    });
    act(() => {
      result.current.add(second);
    });

    expect(result.current.lines.map((l) => l.bookingId)).toEqual(['b1', 'b2']);
    expect(result.current.total).toBe(550000);

    act(() => {
      result.current.remove('b1');
    });
    expect(result.current.lines.map((l) => l.bookingId)).toEqual(['b2']);

    rerender({ due: snapshot([cashier, second]) });
    expect(result.current.lines.map((l) => l.bookingId)).toEqual(['b2']);
  });

  it('persists the cart for this complex, so a reload brings the balances back', () => {
    const { result } = renderHook(() => useSellBalances(COMPLEX, snapshot([cashier])));

    act(() => {
      result.current.add(cashier);
    });

    const { result: reloaded } = renderHook(() => useSellBalances(COMPLEX, snapshot([cashier])));
    expect(reloaded.current.lines.map((l) => l.bookingId)).toEqual(['b1']);
  });
});

describe('useSellBalances — day list refreshes', () => {
  it('drops a line when a refetch shows its balance changed, and reports it', () => {
    const { result, rerender } = renderHook(({ due }) => useSellBalances(COMPLEX, due), {
      initialProps: { due: snapshot([cashier, second]) },
    });
    act(() => {
      result.current.add(cashier);
      result.current.add(second);
    });

    rerender({ due: snapshot([makeBalanceDue({ ...cashier, remaining: 0 }), second], 2) });

    expect(result.current.lines.map((l) => l.bookingId)).toEqual(['b2']);
    expect(result.current.droppedNotice).toBe(true);
  });

  it('keeps every line when a refetch changes nothing', () => {
    const { result, rerender } = renderHook(({ due }) => useSellBalances(COMPLEX, due), {
      initialProps: { due: snapshot([cashier]) },
    });
    act(() => {
      result.current.add(cashier);
    });

    rerender({ due: snapshot([cashier], 2) });

    expect(result.current.lines.map((l) => l.bookingId)).toEqual(['b1']);
    expect(result.current.droppedNotice).toBe(false);
  });
});

describe('useSellBalances — after a successful charge', () => {
  it('removes only the bookings that were charged, keeping a balance added while the charge was in flight', () => {
    const { result } = renderHook(() => useSellBalances(COMPLEX, snapshot([cashier, second])));
    act(() => {
      result.current.add(cashier);
    });

    act(() => {
      result.current.add(second);
      result.current.removeCharged(['b1']);
    });

    expect(result.current.lines.map((l) => l.bookingId)).toEqual(['b2']);
  });
});
