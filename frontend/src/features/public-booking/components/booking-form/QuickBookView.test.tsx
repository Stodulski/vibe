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
  it('ignores a stale non-AR phone_prefix and always submits a +54 number', async () => {
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
      />,
    );

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
      />,
    );

    expect(screen.getByText(/\+54 1123456789/)).toBeInTheDocument();
    expect(screen.queryByText(/\+55/)).not.toBeInTheDocument();
  });
});
