import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { makePair } from '../testing/fixtures';
import { PairsSection } from './PairsSection';
import type { TournamentOwnerPair } from '@/shared/types/api.types';

function renderPair(pair: TournamentOwnerPair) {
  render(<PairsSection pairs={[pair]} groupNames={new Map()} canSwap={false} onSwap={vi.fn()} />);
}

function withRefund(pair: TournamentOwnerPair, status: string, refund: number): TournamentOwnerPair {
  return { ...pair, payment: { status, amount: 2100000, service_fee: 100000, refund_amount: refund } };
}

describe('PairsSection money line', () => {
  it('shows what a confirmed pair paid, in its state badge', () => {
    renderPair(makePair({ id: 'p1' }));
    expect(screen.getByText(/^Pagó \$\s?21\.000$/)).toBeInTheDocument();
    expect(screen.queryByText(/Devuelto/)).not.toBeInTheDocument();
  });

  it('shows what came back on the detail line for a confirmed pair with a partial refund', () => {
    renderPair(withRefund(makePair({ id: 'p1' }), 'deposit_paid', 500000));
    expect(screen.getByText(/^Pagó \$\s?21\.000$/)).toBeInTheDocument();
    expect(screen.getByText(/^Devuelto \$\s?5\.000$/)).toBeInTheDocument();
  });

  it('shows only what came back for a cancelled pair that was refunded', () => {
    renderPair(withRefund(makePair({ id: 'p1', registration: 'cancelled' }), 'refunded', 2100000));
    expect(screen.getByText(/^Devuelto \$\s?21\.000$/)).toBeInTheDocument();
    expect(screen.queryByText(/Pagó/)).not.toBeInTheDocument();
  });

  it('names each pair payment once, even with a partial refund', () => {
    const pairs = [
      withRefund(makePair({ id: 'p1' }), 'deposit_paid', 500000),
      makePair({ id: 'p2', players: ['Carla Ruiz', 'Dana Soto'] }),
    ];
    render(<PairsSection pairs={pairs} groupNames={new Map()} canSwap={false} onSwap={vi.fn()} />);
    const rows = screen.getAllByRole('listitem');
    expect(rows).toHaveLength(2);
    for (const row of rows) {
      expect(within(row).getAllByText(/Pagó/)).toHaveLength(1);
    }
  });
});
