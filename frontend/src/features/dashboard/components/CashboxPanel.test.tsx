import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { CashboxPanel } from './CashboxPanel';
import { renderWithProviders } from '@/test/test-utils';
import { useCashSession } from '@/shared/hooks/useCashSession';
import { makeCashSession } from '@/test/factories';
import type { CashSessionSummary } from '@/shared/types/api.types';

vi.mock('@/shared/hooks/useCashSession', () => ({ useCashSession: vi.fn() }));

function mockLoading() {
  vi.mocked(useCashSession).mockReturnValue({
    data: undefined,
    isLoading: true,
    isRealError: false,
    isClosed: false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useCashSession>);
}

function mockClosed() {
  vi.mocked(useCashSession).mockReturnValue({
    data: undefined,
    isLoading: false,
    isRealError: false,
    isClosed: true,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useCashSession>);
}

function mockOpen(overrides: { expectedCash?: number; openedAt?: string } = {}) {
  const session = makeCashSession({ opened_at: overrides.openedAt ?? '2026-01-05T13:30:00-03:00' });
  const summary: CashSessionSummary = {
    opening_cash: session.opening_cash,
    expected_cash: overrides.expectedCash ?? 650000,
    movement_totals: [],
    booking_payments: [],
  };
  vi.mocked(useCashSession).mockReturnValue({
    data: { cash_session: session, summary },
    isLoading: false,
    isRealError: false,
    isClosed: false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useCashSession>);
}

function mockRealErrorNoData(refetch: () => void) {
  vi.mocked(useCashSession).mockReturnValue({
    data: undefined,
    isLoading: false,
    isRealError: true,
    isClosed: false,
    refetch,
  } as unknown as ReturnType<typeof useCashSession>);
}

function mockRealErrorWithCachedData() {
  const session = makeCashSession();
  const summary: CashSessionSummary = {
    opening_cash: session.opening_cash,
    expected_cash: 200000,
    movement_totals: [],
    booking_payments: [],
  };
  vi.mocked(useCashSession).mockReturnValue({
    data: { cash_session: session, summary },
    isLoading: false,
    isRealError: true,
    isClosed: false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useCashSession>);
}

describe('CashboxPanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows a loading skeleton while the session query is in flight', () => {
    mockLoading();
    renderWithProviders(<CashboxPanel complexId="c1" />);
    expect(screen.queryByText('Caja abierta')).not.toBeInTheDocument();
    expect(screen.queryByText('Caja cerrada')).not.toBeInTheDocument();
  });

  it('shows the closed state with a link to open the till', () => {
    mockClosed();
    renderWithProviders(<CashboxPanel complexId="c1" />);
    expect(screen.getByText('Caja cerrada')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Abrir caja' })).toHaveAttribute('href', '/cash');
  });

  it('shows the open state with expected cash, opened time and a link to the shift', () => {
    mockOpen({ expectedCash: 650000 });
    renderWithProviders(<CashboxPanel complexId="c1" />);
    expect(screen.getByText('Caja abierta')).toBeInTheDocument();
    expect(screen.getByText('$6.500')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Ir a la caja' })).toHaveAttribute('href', '/cash');
  });

  it('shows a compact retry with no full-page takeover when there is no cached data', async () => {
    const user = userEvent.setup();
    const refetch = vi.fn();
    mockRealErrorNoData(refetch);
    renderWithProviders(<CashboxPanel complexId="c1" />);
    expect(screen.queryByText('Caja abierta')).not.toBeInTheDocument();
    expect(screen.queryByText('Caja cerrada')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Reintentar' }));
    expect(refetch).toHaveBeenCalled();
  });

  it('keeps rendering cached data with a non-blocking notice on a background refetch failure', () => {
    mockRealErrorWithCachedData();
    renderWithProviders(<CashboxPanel complexId="c1" />);
    expect(screen.getByText('Caja abierta')).toBeInTheDocument();
    expect(screen.getByRole('status')).toBeInTheDocument();
  });
});
