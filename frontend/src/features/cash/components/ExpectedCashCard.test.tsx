import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ExpectedCashCard } from './ExpectedCashCard';
import { makeCashSession } from '@/test/factories';
import type { CashSessionSummary } from '@/shared/types/api.types';

function makeSummary(overrides: Partial<CashSessionSummary> = {}): CashSessionSummary {
  return {
    opening_cash: 500000,
    expected_cash: 650000,
    movement_totals: [],
    booking_payments: [],
    manual_refunds: [],
    cash_manual_refunds: 0,
    ...overrides,
  };
}

describe('ExpectedCashCard', () => {
  it('hides the cash manual refunds line when there were none in the window', () => {
    const session = makeCashSession();
    render(<ExpectedCashCard session={session} summary={makeSummary({ cash_manual_refunds: 0 })} />);

    expect(screen.queryByTestId('cash-manual-refunds-row')).not.toBeInTheDocument();
    expect(screen.queryByText('Devoluciones en efectivo')).not.toBeInTheDocument();
  });

  it('shows the cash manual refunds line, subtracting, when the session refunded cash by hand', () => {
    const session = makeCashSession();
    render(
      <ExpectedCashCard
        session={session}
        summary={makeSummary({ expected_cash: 400000, cash_manual_refunds: 20000 })}
      />,
    );

    const row = screen.getByTestId('cash-manual-refunds-row');
    expect(row).toHaveTextContent('Devoluciones en efectivo');
    expect(row).toHaveTextContent('-$200');
  });

  it('shows the expected cash amount smaller on mobile and at its full size from sm up', () => {
    const session = makeCashSession();
    render(<ExpectedCashCard session={session} summary={makeSummary()} />);

    const amount = screen.getByText('$6.500');
    expect(amount).toHaveClass('text-lg', 'sm:text-2xl', 'tabular-nums');
  });
});
