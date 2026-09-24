import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import CashPage from './CashPage';
import { useSelectedComplex } from '@/features/complex/hooks/useSelectedComplex';
import { useCashSession } from '@/shared/hooks/useCashSession';
import { useCashSessionDetail } from '../hooks/useCashSessionDetail';
import { useCashSessions } from '../hooks/useCashSessions';
import { makeCashSession, makeCashMovement } from '@/test/factories';
import type { CashSessionSummary } from '@/shared/types/api.types';

vi.mock('@/features/complex/hooks/useSelectedComplex', () => ({ useSelectedComplex: vi.fn() }));
vi.mock('@/shared/hooks/useCashSession', () => ({ useCashSession: vi.fn() }));
vi.mock('../hooks/useCashSessionDetail', () => ({ useCashSessionDetail: vi.fn() }));
vi.mock('../hooks/useCashSessions', () => ({ useCashSessions: vi.fn() }));

function mockClosed() {
  vi.mocked(useCashSession).mockReturnValue({
    data: undefined,
    isLoading: false,
    isError: true,
    isClosed: true,
    isRealError: false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useCashSession>);
  vi.mocked(useCashSessionDetail).mockReturnValue({
    data: undefined,
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useCashSessionDetail>);
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
  vi.mocked(useCashSessionDetail).mockReturnValue({
    data: undefined,
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useCashSessionDetail>);
}

function mockOpen() {
  const session = makeCashSession();
  const summary: CashSessionSummary = {
    opening_cash: session.opening_cash,
    expected_cash: 650000,
    movement_totals: [],
    booking_payments: [],
    manual_refunds: [],
    cash_manual_refunds: 0,
  };
  vi.mocked(useCashSession).mockReturnValue({
    data: { cash_session: session, summary },
    isLoading: false,
    isError: false,
    isClosed: false,
    isRealError: false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useCashSession>);
  vi.mocked(useCashSessionDetail).mockReturnValue({
    data: { cash_session: session, summary, movements: [makeCashMovement()] },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useCashSessionDetail>);
}

/**
 * A background refetch failed (5xx, reconnect) but `detailQuery` still
 * carries the last good data — same shape a real failed React Query
 * background refetch leaves behind (T3 review: "background refetch error
 * wipes the view").
 */
function mockOpenWithStaleDetailError() {
  const session = makeCashSession();
  const summary: CashSessionSummary = {
    opening_cash: session.opening_cash,
    expected_cash: 650000,
    movement_totals: [],
    booking_payments: [],
    manual_refunds: [],
    cash_manual_refunds: 0,
  };
  vi.mocked(useCashSession).mockReturnValue({
    data: { cash_session: session, summary },
    isLoading: false,
    isError: false,
    isClosed: false,
    isRealError: false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useCashSession>);
  vi.mocked(useCashSessionDetail).mockReturnValue({
    data: { cash_session: session, summary, movements: [makeCashMovement()] },
    isLoading: false,
    isError: true,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useCashSessionDetail>);
}

/** Same open-session shape as `mockOpen`, but with a caller-supplied movement list. */
function mockOpenWithMovements(movements: ReturnType<typeof makeCashMovement>[]) {
  const session = makeCashSession();
  const summary: CashSessionSummary = {
    opening_cash: session.opening_cash,
    expected_cash: 650000,
    movement_totals: [],
    booking_payments: [],
    manual_refunds: [],
    cash_manual_refunds: 0,
  };
  vi.mocked(useCashSession).mockReturnValue({
    data: { cash_session: session, summary },
    isLoading: false,
    isError: false,
    isClosed: false,
    isRealError: false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useCashSession>);
  vi.mocked(useCashSessionDetail).mockReturnValue({
    data: { cash_session: session, summary, movements },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useCashSessionDetail>);
}

function renderPage() {
  // `VoidMovementDialog` (always mounted inside the open-session view, even
  // with no target) calls `useVoidCashMovement`, which reaches for a real
  // `useQueryClient()` — not itself mocked here, since it's exercised by its
  // own dedicated tests, but it still needs a provider to mount at all.
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <CashPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('CashPage — empty vs open state', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSelectedComplex).mockReturnValue({
      complex: { id: 'c1' },
      selectedComplexId: 'c1',
    } as unknown as ReturnType<typeof useSelectedComplex>);
    vi.mocked(useCashSessions).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
      hasNextPage: false,
      fetchNextPage: vi.fn(),
      isFetchingNextPage: false,
    } as unknown as ReturnType<typeof useCashSessions>);
  });

  it('shows the closed empty state when no session is open', () => {
    mockClosed();
    renderPage();
    expect(screen.getByText('La caja está cerrada')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Abrir caja' })).toBeInTheDocument();
    expect(screen.queryByText('Efectivo esperado')).not.toBeInTheDocument();
  });

  it('shows the open session view with the expected cash figure when a session is open', () => {
    mockOpen();
    renderPage();
    expect(screen.getByText('Efectivo esperado')).toBeInTheDocument();
    expect(screen.getByText('$6.500')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Ingreso/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Egreso/ })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Cerrar caja/ })).toBeInTheDocument();
    expect(screen.queryByText('La caja está cerrada')).not.toBeInTheDocument();
  });

  it('shows a loading skeleton while the session query is in flight', () => {
    mockLoading();
    renderPage();
    expect(screen.getByRole('status')).toBeInTheDocument();
    expect(screen.queryByText('La caja está cerrada')).not.toBeInTheDocument();
  });
});

// System categories the products/sales delivery writes (never through the
// manual movement form): a session detail carrying a `sale`/`restock`
// movement used to fail schema parsing entirely and show the full-screen
// "No pudimos cargar la caja" error (see cash.schema.test.ts).
describe('CashPage — system categories (sale, restock)', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSelectedComplex).mockReturnValue({
      complex: { id: 'c1' },
      selectedComplexId: 'c1',
    } as unknown as ReturnType<typeof useSelectedComplex>);
    vi.mocked(useCashSessions).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
      hasNextPage: false,
      fetchNextPage: vi.fn(),
      isFetchingNextPage: false,
    } as unknown as ReturnType<typeof useCashSessions>);
  });

  it('renders a sale and a restock movement in the open session, with Anular offered only for the restock', () => {
    mockOpenWithMovements([
      makeCashMovement({ id: 'sale-1', kind: 'income', category: 'sale' }),
      makeCashMovement({ id: 'restock-1', kind: 'expense', category: 'restock' }),
    ]);

    renderPage();

    expect(screen.getByText('Efectivo esperado')).toBeInTheDocument();
    expect(screen.getByText('Venta')).toBeInTheDocument();
    expect(screen.getByText('Reposición')).toBeInTheDocument();
    // Exactly one Anular action offered among the two movements: the sale
    // income is refused with 409 by the backend on a manual void.
    expect(screen.getAllByRole('button', { name: 'Anular' })).toHaveLength(1);
  });
});

describe('CashPage — background refetch failure with cached data', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSelectedComplex).mockReturnValue({
      complex: { id: 'c1' },
      selectedComplexId: 'c1',
    } as unknown as ReturnType<typeof useSelectedComplex>);
    vi.mocked(useCashSessions).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
      hasNextPage: false,
      fetchNextPage: vi.fn(),
      isFetchingNextPage: false,
    } as unknown as ReturnType<typeof useCashSessions>);
  });

  it('keeps the open view, and an open dialog with typed input, mounted instead of the full-screen error', async () => {
    const user = userEvent.setup();
    mockOpenWithStaleDetailError();
    renderPage();

    // The view itself kept rendering off the cached data...
    expect(screen.getByText('Efectivo esperado')).toBeInTheDocument();
    // ...with a non-blocking notice instead of the full-screen error.
    expect(screen.getByText('No pudimos actualizar. Mostrando los últimos datos guardados.')).toBeInTheDocument();
    expect(screen.queryByText('No pudimos cargar la caja')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /Ingreso/ }));
    const incomeDialog = screen.getByRole('dialog');
    await user.type(within(incomeDialog).getByLabelText('Monto'), '1234');

    // The dialog, and what was typed into it, are still mounted — the stale
    // notice never unmounted them.
    expect(incomeDialog).toBeVisible();
    expect(within(incomeDialog).getByLabelText('Monto')).toHaveValue('1.234');
  });
});
