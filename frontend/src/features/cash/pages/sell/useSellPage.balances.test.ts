import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import {
  makeConsumedHttpError,
  makeCashSession,
  makeProduct,
  makeBalanceDue,
  makeSaleBookingPayment,
} from '@/test/factories';
import { useSelectedComplex } from '@/features/complex/hooks/useSelectedComplex';
import { useCashSession } from '@/shared/hooks/useCashSession';
import { useProducts } from '@/shared/hooks/useProducts';
import { useCreateSale } from '../../hooks/useCreateSale';
import { useBookingsDue } from '../../hooks/useBookingsDue';
import { useSellPage } from './useSellPage';

// Mocks the mutation and the day list, then invokes the captured callbacks
// directly — the same shape as `useSellPage.fieldErrors.test.ts`, for pure
// assertions about the checkout payload and its outcome.
vi.mock('@/features/complex/hooks/useSelectedComplex', () => ({ useSelectedComplex: vi.fn() }));
vi.mock('@/shared/hooks/useCashSession', () => ({ useCashSession: vi.fn() }));
vi.mock('@/shared/hooks/useProducts', () => ({ useProducts: vi.fn() }));
vi.mock('../../hooks/useCreateSale', () => ({ useCreateSale: vi.fn() }));
vi.mock('../../hooks/useBookingsDue', () => ({ useBookingsDue: vi.fn() }));

const PRODUCT = makeProduct({ id: 'p1', name: 'Agua', price: 100000 });
const BOOKING = makeBalanceDue({
  booking_id: 'b1',
  court_name: 'Cancha 2',
  start_time: '19:00',
  client_name: 'Juan Pérez',
  remaining: 350000,
});

interface MutateOptions {
  onSuccess: (data: unknown) => void;
  onError: (error: unknown) => void;
}

function mockMutation() {
  const mutate = vi.fn();
  vi.mocked(useCreateSale).mockReturnValue({ mutate, isPending: false } as unknown as ReturnType<typeof useCreateSale>);
  return mutate;
}

async function mountWithDayList() {
  const hook = renderHook(() => useSellPage());
  await waitFor(() => {
    expect(hook.result.current.balanceDue.bookings).toHaveLength(1);
  });
  return hook;
}

beforeEach(() => {
  vi.clearAllMocks();
  window.sessionStorage.clear();
  vi.mocked(useSelectedComplex).mockReturnValue({
    complex: { id: 'c1' },
    selectedComplexId: 'c1',
  } as unknown as ReturnType<typeof useSelectedComplex>);
  vi.mocked(useCashSession).mockReturnValue({
    data: {
      cash_session: makeCashSession(),
      summary: { opening_cash: 0, expected_cash: 0, movement_totals: [], booking_payments: [] },
    },
    isLoading: false,
    isError: false,
    isClosed: false,
    isRealError: false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useCashSession>);
  vi.mocked(useProducts).mockReturnValue({
    data: { products: [PRODUCT] },
    isLoading: false,
    isError: false,
    isSuccess: true,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useProducts>);
  vi.mocked(useBookingsDue).mockReturnValue({
    data: { date: '2026-10-12', bookings: [BOOKING] },
    isSuccess: true,
    isLoading: false,
    isError: false,
    dataUpdatedAt: 1,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useBookingsDue>);
});

describe('useSellPage — checkout with booking balances', () => {
  it('a balance-only checkout sends no products and the booking at its remaining amount', async () => {
    const mutate = mockMutation();
    const { result } = await mountWithDayList();

    act(() => {
      result.current.addBalance(BOOKING);
    });
    expect(result.current.total).toBe(350000);

    act(() => {
      result.current.charge();
    });

    expect(mutate).toHaveBeenCalledTimes(1);
    expect(mutate.mock.calls[0]?.[0]).toEqual({
      items: [],
      booking_payments: [{ booking_id: 'b1', amount: 350000 }],
      method: 'cash',
      note: undefined,
    });
  });

  it('a mixed checkout sends the product lines and the balances together, and totals both', async () => {
    const mutate = mockMutation();
    const { result } = await mountWithDayList();

    act(() => {
      result.current.addProduct(PRODUCT);
      result.current.addBalance(BOOKING);
    });
    expect(result.current.total).toBe(450000);

    act(() => {
      result.current.charge();
    });

    expect(mutate.mock.calls[0]?.[0]).toMatchObject({
      items: [{ product_id: 'p1', quantity: 1 }],
      booking_payments: [{ booking_id: 'b1', amount: 350000 }],
    });
  });
});

describe('useSellPage — checkout outcome with booking balances', () => {
  it('a successful balance-only checkout shows the result without a sale and clears the collected balance', async () => {
    const mutate = mockMutation();
    const { result } = await mountWithDayList();
    act(() => {
      result.current.addBalance(BOOKING);
    });
    act(() => {
      result.current.charge();
    });
    const [, options] = mutate.mock.calls[0] as [unknown, MutateOptions];

    act(() => {
      options.onSuccess({
        sale: null,
        stock_warnings: [],
        booking_payments: [makeSaleBookingPayment({ booking_id: 'b1', amount: 350000 })],
      });
    });

    expect(result.current.saleResult?.sale).toBeNull();
    expect(result.current.saleResult?.bookingPayments).toHaveLength(1);
    expect(result.current.saleResult?.method).toBe('cash');
    expect(result.current.balanceLines).toEqual([]);
  });

  it('a 422 on booking_payments[0] lands on that balance line, translated to Spanish', async () => {
    const mutate = mockMutation();
    const { result } = await mountWithDayList();
    act(() => {
      result.current.addBalance(BOOKING);
    });
    act(() => {
      result.current.charge();
    });
    const [, options] = mutate.mock.calls[0] as [unknown, MutateOptions];

    await act(async () => {
      options.onError(
        await makeConsumedHttpError(422, {
          title: 'Validation failed',
          errors: [{ field: 'booking_payments[0].booking_id', message: 'booking not found in this complex' }],
        }),
      );
    });

    expect(result.current.lineErrors.b1).toBe('El turno no pertenece a este complejo.');
    expect(result.current.balanceLines).toHaveLength(1);
  });
});

describe('useSellPage — a booking changed before the charge landed (409)', () => {
  it.each([
    "a booking's balance changed since the cart was loaded, reload the balances and try again",
    'this booking is already fully paid',
    'this booking cannot take a payment: it is cancelled, a no-show, or not confirmed yet',
  ])('shows the notice for "%s" and keeps the rest of the cart', async (detail) => {
    const mutate = mockMutation();
    const { result } = await mountWithDayList();
    act(() => {
      result.current.addProduct(PRODUCT);
      result.current.addBalance(BOOKING);
    });
    act(() => {
      result.current.charge();
    });
    const [, options] = mutate.mock.calls[0] as [unknown, MutateOptions];

    await act(async () => {
      options.onError(await makeConsumedHttpError(409, { title: 'Conflict', detail }));
    });

    expect(result.current.balanceChangedNotice).toBe(true);
    expect(result.current.lines).toHaveLength(1);
  });

  it('a 409 without a booking reason (no open session) does not raise the notice', async () => {
    const mutate = mockMutation();
    const { result } = await mountWithDayList();
    act(() => {
      result.current.addBalance(BOOKING);
    });
    act(() => {
      result.current.charge();
    });
    const [, options] = mutate.mock.calls[0] as [unknown, MutateOptions];

    await act(async () => {
      options.onError(await makeConsumedHttpError(409, { title: 'Conflict', detail: 'no cash session is open' }));
    });

    expect(result.current.balanceChangedNotice).toBe(false);
  });
});
