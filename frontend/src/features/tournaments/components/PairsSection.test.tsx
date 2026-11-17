import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { userEvent } from '@/test/test-utils';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import { makePair } from '../testing/fixtures';
import { PairsSection } from './PairsSection';
import type { TournamentOwnerPair } from '@/shared/types/api.types';

const t = ES_AR.tournaments.pairs;
const PRICE = 2000000;

function renderPairs(pairs: TournamentOwnerPair[], opts: { seeded?: boolean } = {}) {
  render(
    <PairsSection
      pairs={pairs}
      groupNames={new Map([['g1', 'B']])}
      canSwap={false}
      onSwap={vi.fn()}
      price={PRICE}
      seeded={opts.seeded ?? false}
    />,
  );
}

function withPayment(pair: TournamentOwnerPair, status: string, amount: number, refund = 0): TournamentOwnerPair {
  return { ...pair, payment: { status, amount, service_fee: 100000, refund_amount: refund } };
}

describe('PairsSection payment badge', () => {
  it('shows no badge for a pair confirmed at the category price', () => {
    renderPairs([makePair({ id: 'p1' })]);
    expect(screen.queryByText(/Pagó/)).not.toBeInTheDocument();
  });

  it('shows what a pair paid when it differs from the category price', () => {
    renderPairs([withPayment(makePair({ id: 'p1' }), 'approved', 1000000)]);
    expect(screen.getByText(t.paid(formatPrice(1000000)))).toBeInTheDocument();
  });

  it('shows the state of a pair still paying', () => {
    renderPairs([makePair({ id: 'p1', registration: 'held' })]);
    expect(screen.getByText(t.state.held)).toBeInTheDocument();
  });

  it('keeps what was paid and what came back for a partial refund', () => {
    renderPairs([withPayment(makePair({ id: 'p1' }), 'deposit_paid', PRICE, 500000)]);
    expect(screen.getByText(t.paid(formatPrice(PRICE)))).toBeInTheDocument();
    expect(screen.getByText(t.refundedAmount(formatPrice(500000)))).toBeInTheDocument();
  });

  it('shows the refund state of a cancelled pair that was refunded', () => {
    renderPairs([withPayment(makePair({ id: 'p1', registration: 'cancelled' }), 'refunded', PRICE, PRICE)]);
    expect(screen.getByText(t.state.refunded)).toBeInTheDocument();
    expect(screen.getByText(t.refundedAmount(formatPrice(PRICE)))).toBeInTheDocument();
  });
});

describe('PairsSection row', () => {
  it('shows the zone and seed under the names when they are set', () => {
    renderPairs([makePair({ id: 'p1', seed: 7, groupId: 'g1' })]);
    expect(screen.getByText('Zona B · Serie 7')).toBeInTheDocument();
  });

  it('shows each player’s declared gender and category on the detail line', () => {
    renderPairs([
      makePair({
        id: 'p1',
        players: ['Ana Pérez', 'Bea Gómez'],
        declared: [
          { gender: 'men', level: 7 },
          { gender: 'women', level: 6 },
        ],
      }),
    ]);
    expect(screen.getByText('Caballero · 7ma / Dama · 6ta')).toBeInTheDocument();
  });

  it('shows nothing extra for a pair that signed up before the traits', () => {
    renderPairs([makePair({ id: 'p1', players: ['Ana Pérez', 'Bea Gómez'] })]);
    expect(screen.queryByText(/Caballero|Dama/)).not.toBeInTheDocument();
  });

  it('keeps the phones out of the row and in the contact menu', async () => {
    const user = userEvent.setup();
    renderPairs([makePair({ id: 'p1', players: ['Ana Pérez', 'Bea Gómez'] })]);

    expect(screen.queryByText('1155550000')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', { name: t.contact }));
    expect(await screen.findByRole('menuitem', { name: `${t.call} Ana Pérez` })).toHaveAttribute(
      'href',
      'tel:1155550000',
    );
    expect(screen.getByRole('menuitem', { name: t.whatsappAria('Bea Gómez') })).toHaveAttribute(
      'href',
      'https://wa.me/1155550000',
    );
  });
});

describe('PairsSection order', () => {
  it('keeps the given order before the draw and follows the seeds once it exists', () => {
    const pairs = [
      makePair({ id: 'p1', players: ['Ana Pérez', 'Bea Gómez'] }),
      makePair({ id: 'p2', players: ['Carla Ruiz', 'Dana Soto'], seed: 2 }),
      makePair({ id: 'p3', players: ['Eva Díaz', 'Flor Vega'], seed: 1 }),
    ];

    const { unmount } = render(
      <PairsSection
        pairs={pairs}
        groupNames={new Map()}
        canSwap={false}
        onSwap={vi.fn()}
        price={PRICE}
        seeded={false}
      />,
    );
    expect(screen.getAllByRole('listitem').map((row) => row.textContent)[0]).toContain('Ana Pérez');
    unmount();

    renderPairs(pairs, { seeded: true });
    const rows = screen.getAllByRole('listitem').map((row) => row.textContent);
    expect(rows[0]).toContain('Eva Díaz');
    expect(rows[1]).toContain('Carla Ruiz');
    expect(rows[2]).toContain('Ana Pérez');
  });
});
