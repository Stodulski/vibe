import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
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
  it('shows what a confirmed pair paid', () => {
    renderPair(makePair({ id: 'p1' }));
    expect(screen.getByText(/^Pagó \$\s?21\.000$/)).toBeInTheDocument();
    expect(screen.queryByText(/Devuelto/)).not.toBeInTheDocument();
  });

  it('shows both what was paid and what came back for a confirmed pair with a partial refund', () => {
    renderPair(withRefund(makePair({ id: 'p1' }), 'deposit_paid', 500000));
    expect(screen.getByText(/^Pagó \$\s?21\.000 · Devuelto \$\s?5\.000$/)).toBeInTheDocument();
  });

  it('shows only what came back for a cancelled pair that was refunded', () => {
    renderPair(withRefund(makePair({ id: 'p1', registration: 'cancelled' }), 'refunded', 2100000));
    expect(screen.getByText(/^Devuelto \$\s?21\.000$/)).toBeInTheDocument();
    expect(screen.queryByText(/Pagó/)).not.toBeInTheDocument();
  });
});
