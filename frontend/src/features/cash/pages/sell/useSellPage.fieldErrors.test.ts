import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { makeConsumedHttpError, makeCashSession, makeProduct } from '@/test/factories';
import { useSelectedComplex } from '@/features/complex/hooks/useSelectedComplex';
import { useCashSession } from '@/shared/hooks/useCashSession';
import { useProducts } from '@/shared/hooks/useProducts';
import { useCreateSale } from '../../hooks/useCreateSale';
import { useSellPage } from './useSellPage';

// Same "mock the mutation hook, invoke its captured onError directly" shape
// as `OpenCashSessionDialog.test.tsx` — more reliable than round-tripping a
// real 422 through ky/MSW for a pure field-mapping assertion.
vi.mock('@/features/complex/hooks/useSelectedComplex', () => ({ useSelectedComplex: vi.fn() }));
vi.mock('@/shared/hooks/useCashSession', () => ({ useCashSession: vi.fn() }));
vi.mock('@/shared/hooks/useProducts', () => ({ useProducts: vi.fn() }));
vi.mock('../../hooks/useCreateSale', () => ({ useCreateSale: vi.fn() }));

const PRODUCT = makeProduct({ id: 'p1', name: 'Agua', price: 100000 });

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
});

describe('useSellPage — 422 line mapping', () => {
  it('maps a per-item field error onto the matching cart line and translates it to Spanish', async () => {
    const mutate = vi.fn();
    vi.mocked(useCreateSale).mockReturnValue({ mutate, isPending: false } as unknown as ReturnType<
      typeof useCreateSale
    >);

    const { result } = renderHook(() => useSellPage());
    await waitFor(() => {
      expect(result.current.visibleProducts).toHaveLength(1);
    });

    act(() => {
      result.current.addProduct(PRODUCT);
    });
    act(() => {
      result.current.charge();
    });

    expect(mutate).toHaveBeenCalledTimes(1);
    const [, mutateOptions] = mutate.mock.calls[0] as [unknown, { onError: (error: unknown) => void }];

    await act(async () => {
      mutateOptions.onError(
        await makeConsumedHttpError(422, {
          title: 'Validation failed',
          errors: [{ field: 'items[0].product_id', message: 'product not found or not active' }],
        }),
      );
    });

    expect(result.current.lineErrors.p1).toBe('Este producto ya no está disponible para vender.');
    // The cart itself is left alone — the owner removes the offending line.
    expect(result.current.lines).toEqual([{ productId: 'p1', quantity: 1 }]);
  });

  it('clears previous line errors on a new charge attempt', async () => {
    const mutate = vi.fn();
    vi.mocked(useCreateSale).mockReturnValue({ mutate, isPending: false } as unknown as ReturnType<
      typeof useCreateSale
    >);
    const { result } = renderHook(() => useSellPage());
    await waitFor(() => {
      expect(result.current.visibleProducts).toHaveLength(1);
    });

    act(() => {
      result.current.addProduct(PRODUCT);
    });
    act(() => {
      result.current.charge();
    });
    const [, firstOptions] = mutate.mock.calls[0] as [unknown, { onError: (error: unknown) => void }];
    await act(async () => {
      firstOptions.onError(
        await makeConsumedHttpError(422, {
          errors: [{ field: 'items[0].product_id', message: 'product not found or not active' }],
        }),
      );
    });
    expect(result.current.lineErrors.p1).toBeDefined();

    act(() => {
      result.current.charge();
    });
    expect(result.current.lineErrors).toEqual({});
  });
});
