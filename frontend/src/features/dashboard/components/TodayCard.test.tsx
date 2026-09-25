import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TodayCard } from './TodayCard';
import { renderWithProviders } from '@/test/test-utils';
import { useCashSession } from '@/shared/hooks/useCashSession';
import { makeCashSession } from '@/test/factories';
import type { CashSessionSummary, DashboardStats } from '@/shared/types/api.types';

vi.mock('@/shared/hooks/useCashSession', () => ({ useCashSession: vi.fn() }));

function makeStats(overrides: Partial<DashboardStats['today_money']> = {}): DashboardStats {
  return {
    today_bookings: 0,
    yesterday_bookings: 0,
    today_revenue: 0,
    yesterday_revenue: 0,
    weekly_revenue: 0,
    monthly_revenue: 0,
    occupancy_rate: 0,
    pending_bookings: 0,
    total_clients: 0,
    upcoming_bookings: [],
    payment_summary: { by_status: {}, by_method: {} },
    today_money: {
      bookings: 100000,
      bar_sales: 50000,
      other_income: 20000,
      expenses: 15000,
      total_income: 170000,
      by_method: { cash: 90000, transfer: 80000 },
      ...overrides,
    },
  };
}

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
    manual_refunds: [],
    cash_manual_refunds: 0,
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
    manual_refunds: [],
    cash_manual_refunds: 0,
  };
  vi.mocked(useCashSession).mockReturnValue({
    data: { cash_session: session, summary },
    isLoading: false,
    isRealError: true,
    isClosed: false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useCashSession>);
}

describe('TodayCard', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows a till skeleton while the session query is in flight, income is still shown', () => {
    mockLoading();
    renderWithProviders(<TodayCard complexId="c1" stats={makeStats()} />);
    expect(screen.queryByText('Abierta')).not.toBeInTheDocument();
    expect(screen.queryByText('Cerrada')).not.toBeInTheDocument();
    // The income half does not depend on the till query, so it renders
    // immediately even while the till is still loading.
    expect(screen.getByText('$1.700')).toBeInTheDocument();
  });

  it('shows the closed till state with a link to open it', () => {
    mockClosed();
    renderWithProviders(<TodayCard complexId="c1" stats={makeStats()} />);
    expect(screen.getByText('Cerrada')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Abrir caja' })).toHaveAttribute('href', '/cash');
  });

  it('shows the open till state with expected cash and a link to the shift', () => {
    mockOpen({ expectedCash: 650000 });
    renderWithProviders(<TodayCard complexId="c1" stats={makeStats()} />);
    expect(screen.getByText('Abierta')).toBeInTheDocument();
    expect(screen.getByText('$6.500')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Ir a la caja' })).toHaveAttribute('href', '/cash');
  });

  it('shows a compact retry with no full-page takeover when there is no cached till data', async () => {
    const user = userEvent.setup();
    const refetch = vi.fn();
    mockRealErrorNoData(refetch);
    renderWithProviders(<TodayCard complexId="c1" stats={makeStats()} />);
    expect(screen.queryByText('Abierta')).not.toBeInTheDocument();
    expect(screen.queryByText('Cerrada')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: 'Reintentar' }));
    expect(refetch).toHaveBeenCalled();
  });

  it('keeps rendering cached till data with a non-blocking notice on a background refetch failure', () => {
    mockRealErrorWithCachedData();
    renderWithProviders(<TodayCard complexId="c1" stats={makeStats()} />);
    expect(screen.getByText('Abierta')).toBeInTheDocument();
    expect(screen.getByRole('status')).toBeInTheDocument();
  });
});

describe('TodayCard — income', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders every income line and the total', () => {
    mockClosed();
    renderWithProviders(
      <TodayCard
        complexId="c1"
        stats={makeStats({
          bookings: 100000,
          bar_sales: 50000,
          other_income: 20000,
          expenses: 15000,
          total_income: 170000,
        })}
      />,
    );
    expect(screen.getByText('$1.700')).toBeInTheDocument(); // total_income
    expect(screen.getByText('Turnos')).toBeInTheDocument();
    expect(screen.getByText('Bar')).toBeInTheDocument();
    expect(screen.getByText('Otros')).toBeInTheDocument();
    expect(screen.getByText('$1.000')).toBeInTheDocument(); // bookings
    expect(screen.getByText('$500')).toBeInTheDocument(); // bar_sales
    expect(screen.getByText('$200')).toBeInTheDocument(); // other_income
    expect(screen.getByText('$150')).toBeInTheDocument(); // expenses
  });

  it('renders the by-method breakdown, including mercadopago', () => {
    mockClosed();
    renderWithProviders(
      <TodayCard
        complexId="c1"
        stats={makeStats({ by_method: { cash: 90000, transfer: 80000, mercadopago: 40000 } })}
      />,
    );
    expect(screen.getByText('Método de pago')).toBeInTheDocument();
    expect(screen.getByText('Efectivo')).toBeInTheDocument();
    expect(screen.getByText('Transferencia')).toBeInTheDocument();
    expect(screen.getByText('MercadoPago')).toBeInTheDocument();
  });
});
