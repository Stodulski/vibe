import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { http, HttpResponse } from 'msw';
import { server } from '@/test/msw/server';
import CashSellPage from './CashSellPage';
import { useSelectedComplex } from '@/features/complex/hooks/useSelectedComplex';
import { useCashSession } from '@/shared/hooks/useCashSession';
import { useProducts } from '@/shared/hooks/useProducts';
import { makeCashSession, makeProduct } from '@/test/factories';

vi.mock('@/features/complex/hooks/useSelectedComplex', () => ({ useSelectedComplex: vi.fn() }));
vi.mock('@/shared/hooks/useCashSession', () => ({ useCashSession: vi.fn() }));
vi.mock('@/shared/hooks/useProducts', () => ({ useProducts: vi.fn() }));

function mockClosed() {
  vi.mocked(useCashSession).mockReturnValue({
    data: undefined,
    isLoading: false,
    isError: true,
    isClosed: true,
    isRealError: false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useCashSession>);
}

function mockLoading() {
  vi.mocked(useCashSession).mockReturnValue({
    data: undefined,
    isLoading: true,
    isError: false,
    isClosed: false,
    isRealError: false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useCashSession>);
}

function mockRealError() {
  vi.mocked(useCashSession).mockReturnValue({
    data: undefined,
    isLoading: false,
    isError: true,
    isClosed: false,
    isRealError: true,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useCashSession>);
}

function mockOpen() {
  const session = makeCashSession();
  vi.mocked(useCashSession).mockReturnValue({
    data: {
      cash_session: session,
      summary: { opening_cash: 0, expected_cash: 0, movement_totals: [], booking_payments: [] },
    },
    isLoading: false,
    isError: false,
    isClosed: false,
    isRealError: false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useCashSession>);
}

/** Same stub as `ClientGrid.test.tsx`'s `mockViewport`: forces `useMediaQuery` without touching the real viewport. */
function mockViewport(matches: boolean) {
  const spy = vi.spyOn(window, 'matchMedia').mockReturnValue({
    matches,
    media: '',
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  } as unknown as MediaQueryList);
  return () => {
    spy.mockRestore();
  };
}

function renderPage() {
  server.use(
    http.get('*/complexes/:complexId/sales', () => HttpResponse.json({ sales: [], metadata: { has_more: false } })),
  );
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <CashSellPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('CashSellPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSelectedComplex).mockReturnValue({
      complex: { id: 'c1' },
      selectedComplexId: 'c1',
    } as unknown as ReturnType<typeof useSelectedComplex>);
    vi.mocked(useProducts).mockReturnValue({
      data: { products: [] },
      isLoading: false,
      isError: false,
      isSuccess: true,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useProducts>);
  });

  it('shows the closed-till gate with a link to Turno when no session is open', () => {
    mockClosed();
    renderPage();
    expect(screen.getByText('Abrí la caja para vender')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Ir a Turno' })).toHaveAttribute('href', '/cash');
  });

  it('shows a loading skeleton while the session query is in flight', () => {
    mockLoading();
    renderPage();
    expect(screen.getByRole('status')).toBeInTheDocument();
    expect(screen.queryByText('Abrí la caja para vender')).not.toBeInTheDocument();
  });

  it('shows the full-screen load error only when there is nothing cached', () => {
    mockRealError();
    renderPage();
    expect(screen.getByText('No pudimos cargar el catálogo')).toBeInTheDocument();
  });

  it('shows the product grid once the till is open, and tapping a tile adds it to the cart', async () => {
    const user = userEvent.setup();
    vi.mocked(useProducts).mockReturnValue({
      data: { products: [makeProduct({ id: 'p1', name: 'Agua', price: 100000 })] },
      isLoading: false,
      isError: false,
      isSuccess: true,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useProducts>);
    mockOpen();
    const restoreViewport = mockViewport(true);
    renderPage();

    expect(screen.getByRole('button', { name: 'Agua' })).toBeInTheDocument();
    expect(screen.getByText('El carrito está vacío. Tocá un producto para agregarlo.')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Agua' }));

    expect(screen.queryByText('El carrito está vacío. Tocá un producto para agregarlo.')).not.toBeInTheDocument();
    expect(screen.getByTestId('sell-cart-total')).toHaveTextContent('$1.000');
    restoreViewport();
  });
});
