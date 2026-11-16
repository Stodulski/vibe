import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ParticipantList } from './ParticipantList';
import type { OpenPlayParticipant } from '@/shared/types/api.types';

function participant(overrides: Partial<OpenPlayParticipant>): OpenPlayParticipant {
  return {
    id: 'r1',
    first_name: 'Ana',
    last_name: 'Gomez',
    phone: '+5491155550001',
    status: 'confirmed',
    price: 180000,
    service_fee: 20000,
    created_at: '2026-11-14T12:00:00Z',
    payment: { status: 'deposit_paid', amount: 200000, service_fee: 20000, refund_amount: 0, method: 'mercadopago' },
    ...overrides,
  };
}

describe('ParticipantList money line', () => {
  it('shows what a confirmed participant paid', () => {
    render(<ParticipantList participants={[participant({})]} />);
    expect(screen.getByText(/^Pagó \$\s?2\.000$/)).toBeInTheDocument();
    expect(screen.queryByText(/Devuelto/)).not.toBeInTheDocument();
  });

  it('shows both what was paid and what came back for a confirmed participant with a partial refund', () => {
    render(
      <ParticipantList
        participants={[
          participant({
            payment: {
              status: 'deposit_paid',
              amount: 200000,
              service_fee: 20000,
              refund_amount: 50000,
              method: 'mercadopago',
            },
          }),
        ]}
      />,
    );
    expect(screen.getByText(/^Pagó \$\s?2\.000 · Devuelto \$\s?500$/)).toBeInTheDocument();
  });

  it('shows only what came back for a cancelled participant who was refunded', () => {
    render(
      <ParticipantList
        participants={[
          participant({
            status: 'cancelled',
            payment: {
              status: 'refunded',
              amount: 200000,
              service_fee: 20000,
              refund_amount: 200000,
              method: 'mercadopago',
            },
          }),
        ]}
      />,
    );
    expect(screen.getByText(/^Devuelto \$\s?2\.000$/)).toBeInTheDocument();
    expect(screen.queryByText(/Pagó/)).not.toBeInTheDocument();
  });
});
