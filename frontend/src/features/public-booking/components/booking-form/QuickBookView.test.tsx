import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QuickBookView } from './QuickBookView';
import type { BookingSlotInfo } from './types';
import type { BookingPricing } from './pricing';

const slotInfo: BookingSlotInfo = {
  complexId: 'c1',
  complexName: 'Padel Club Norte',
  complexPhone: '1155550000',
  courtId: 'ct1',
  courtName: 'Cancha 1',
  date: '2026-03-20',
  startTime: '10:00',
  endTime: '11:30',
  durationMinutes: 90,
  price: 1500000,
  depositPercentage: 30,
  cancellationHours: 24,
};

const pricing: BookingPricing = {
  depositAmount: 450000,
  hasDeposit: true,
  mpAmount: 450000,
  serviceFee: 31500,
  totalOnline: 481500,
  remainingAmount: 1050000,
};

describe('QuickBookView phone prefix', () => {
  // Regression: before this app fixed the country prefix to Argentina, a
  // returning client's localStorage record could carry a non-AR
  // `phone_prefix` (e.g. from a since-removed selector). That field must
  // never be trusted again — the app only ever sends/stores `+54` numbers.
  it('ignores a stale non-AR phone_prefix and always submits a +54 number, once confirmed', async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();

    render(
      <QuickBookView
        slotInfo={slotInfo}
        pricing={pricing}
        saved={{
          client_first_name: 'Juan',
          client_last_name: 'Perez',
          client_phone: '1123456789',
          client_email: 'juan@example.com',
          phone_prefix: '+55',
        }}
        isLoading={false}
        onSubmit={onSubmit}
        onEdit={vi.fn()}
        onNotMe={vi.fn()}
      />,
    );

    await user.click(screen.getByRole('button', { name: /sí, soy yo/i }));
    await user.click(screen.getByRole('button', { name: /pag/i }));

    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ client_phone: '+541123456789' }));
  });

  it('always displays the fixed +54 prefix in the summary, regardless of a stale saved prefix', () => {
    render(
      <QuickBookView
        slotInfo={slotInfo}
        pricing={pricing}
        saved={{
          client_first_name: 'Juan',
          client_last_name: 'Perez',
          client_phone: '1123456789',
          client_email: 'juan@example.com',
          phone_prefix: '+55',
        }}
        isLoading={false}
        onSubmit={vi.fn()}
        onEdit={vi.fn()}
        onNotMe={vi.fn()}
      />,
    );

    expect(screen.getByText(/\+54 1123456789/)).toBeInTheDocument();
    expect(screen.queryByText(/\+55/)).not.toBeInTheDocument();
  });
});

// SEC-05: quick-book used to submit a saved identity with one tap and no
// confirmation — on a shared/public device, the next visitor's booking went
// out as whoever used it last. An explicit "Sí, soy yo" is now required
// before anything can be submitted, and "No, soy otra persona" is an equally
// reachable way out.
describe('QuickBookView identity confirmation', () => {
  const saved = {
    client_first_name: 'Juan',
    client_last_name: 'Perez',
    client_phone: '1123456789',
    client_email: 'juan@example.com',
  };

  it('renders no submit/pay button before the identity is confirmed', () => {
    render(
      <QuickBookView
        slotInfo={slotInfo}
        pricing={pricing}
        saved={saved}
        isLoading={false}
        onSubmit={vi.fn()}
        onEdit={vi.fn()}
        onNotMe={vi.fn()}
      />,
    );

    expect(screen.queryByRole('button', { name: /pag/i })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /sí, soy yo/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /no, soy otra persona/i })).toBeInTheDocument();
  });

  it('reveals the submit button and lets it submit once "Sí, soy yo" is clicked', async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    render(
      <QuickBookView
        slotInfo={slotInfo}
        pricing={pricing}
        saved={saved}
        isLoading={false}
        onSubmit={onSubmit}
        onEdit={vi.fn()}
        onNotMe={vi.fn()}
      />,
    );

    await user.click(screen.getByRole('button', { name: /sí, soy yo/i }));
    const payButton = screen.getByRole('button', { name: /pag/i });
    await user.click(payButton);

    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ client_first_name: 'Juan', client_email: 'juan@example.com' }),
    );
  });
});

// A sibling describe, not nested in the one above: max-lines-per-function
// counts a describe callback's whole body.
describe('QuickBookView identity confirmation, "No, soy otra persona"', () => {
  const saved = {
    client_first_name: 'Juan',
    client_last_name: 'Perez',
    client_phone: '1123456789',
    client_email: 'juan@example.com',
  };

  it('calls onNotMe and never onSubmit when "No, soy otra persona" is clicked', async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    const onNotMe = vi.fn();
    render(
      <QuickBookView
        slotInfo={slotInfo}
        pricing={pricing}
        saved={saved}
        isLoading={false}
        onSubmit={onSubmit}
        onEdit={vi.fn()}
        onNotMe={onNotMe}
      />,
    );

    await user.click(screen.getByRole('button', { name: /no, soy otra persona/i }));

    expect(onNotMe).toHaveBeenCalledTimes(1);
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
