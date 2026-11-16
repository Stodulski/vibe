import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import type { CancelInfoResponse } from '../../api/types';
import { CancelForm } from './CancelForm';

const cancelInfo = {
  booking: {
    status: 'confirmed',
    date: '2026-03-20',
    start_time: '10:00',
    court_name: 'Cancha 1',
    complex_name: 'Club Norte',
  },
  can_cancel: true,
  can_refund: true,
  refund_method: 'mercadopago',
  cancellation_hours: 24,
} as unknown as CancelInfoResponse;

describe('CancelForm', () => {
  it.each([true, false])('dismisses the confirmation with "Volver" (can refund: %s)', (canRefund) => {
    render(
      <CancelForm
        cancelInfo={cancelInfo}
        canRefund={canRefund}
        confirmOpen
        isCancelling={false}
        onOpenConfirm={vi.fn()}
        onCloseConfirm={vi.fn()}
        onConfirmCancel={vi.fn()}
        onBack={vi.fn()}
      />,
    );
    const dialog = screen.getByRole('alertdialog');
    expect(within(dialog).getByRole('button', { name: 'Volver' })).toBeInTheDocument();
    expect(within(dialog).queryByRole('button', { name: 'Cancelar' })).not.toBeInTheDocument();
  });
});
