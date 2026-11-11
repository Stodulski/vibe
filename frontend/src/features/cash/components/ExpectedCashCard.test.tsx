import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ExpectedCashCard } from './ExpectedCashCard';
import { makeCashMovement, makeCashSession } from '@/test/factories';
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

  it('breaks expected cash into its cash terms, leaving out a cancelled movement', () => {
    const session = makeCashSession({ opening_cash: 500000 });
    const movements = [
      makeCashMovement({ id: 'a', kind: 'income', method: 'cash', amount: 300000 }),
      makeCashMovement({ id: 'b', kind: 'income', method: 'cash', amount: 99900 }),
      makeCashMovement({ id: 'c', kind: 'expense', method: 'cash', amount: 99900, voids_movement_id: 'b' }),
    ];
    render(
      <ExpectedCashCard session={session} summary={makeSummary({ expected_cash: 800000 })} movements={movements} />,
    );

    expect(screen.getByText('Ingresos').nextSibling).toHaveTextContent('$3.000');
    expect(screen.getByText('Egresos').nextSibling).toHaveTextContent('$0');
    expect(screen.getByText('$8.000')).toBeInTheDocument();
  });
});
