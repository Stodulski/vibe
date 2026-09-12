import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ConfirmPaymentModal } from './ConfirmPaymentModal';
import { makeBooking } from '@/test/factories';
import type { Booking } from '@/shared/types/api.types';

const baseBooking: Booking = makeBooking({
  id: 'b1',
  complex_id: 'c1',
  court_id: 'ct1',
  client_id: 'cl1',
  court_name: 'Cancha 1',
  client_name: 'Juan Perez',
  client_phone: '1155550000',
  date: '2026-03-15',
  start_time: '10:00',
  duration_minutes: 90,
  price: 1500000,
  deposit_amount: 0,
  status: 'confirmed',
  collection_status: 'unpaid',
  notes: '',
  reminder_sent_2h: false,
  created_at: '2026-03-14T10:00:00Z',
  updated_at: '2026-03-14T10:00:00Z',
});

describe('ConfirmPaymentModal', () => {
  const defaultProps = {
    open: true,
    onClose: vi.fn(),
    onConfirm: vi.fn(),
    booking: baseBooking,
    isLoading: false,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders payment confirmation title', () => {
    render(<ConfirmPaymentModal {...defaultProps} />);
    // Title is "Confirmar pago" -- there's also a submit button with same text
    expect(screen.getAllByText(/confirmar pago/i).length).toBeGreaterThanOrEqual(1);
  });

  it('renders cancel and submit buttons', () => {
    render(<ConfirmPaymentModal {...defaultProps} />);
    expect(screen.getByRole('button', { name: /cancelar/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /confirmar pago/i })).toBeInTheDocument();
  });

  it('calls onClose when cancel button is clicked', async () => {
    const user = userEvent.setup();
    render(<ConfirmPaymentModal {...defaultProps} />);
    await user.click(screen.getByRole('button', { name: /cancelar/i }));
    expect(defaultProps.onClose).toHaveBeenCalled();
  });

  it('disables submit button when loading', () => {
    render(<ConfirmPaymentModal {...defaultProps} isLoading />);
    const submitBtn = document.querySelector('button[type="submit"]');
    expect(submitBtn).toBeDisabled();
  });

  it('renders remaining balance title when deposit is already paid', () => {
    const depositPaidBooking = {
      ...baseBooking,
      collection_status: 'deposit_paid' as const,
      deposit_amount: 500000,
    };
    render(<ConfirmPaymentModal {...defaultProps} booking={depositPaidBooking} />);
    expect(screen.getByText(/confirmar pago restante/i)).toBeInTheDocument();
  });

  it('renders form when booking is null (no booking info block shown)', () => {
    render(<ConfirmPaymentModal {...defaultProps} booking={null} />);
    // The dialog renders with form elements even without booking
    expect(screen.getAllByText(/confirmar pago/i).length).toBeGreaterThanOrEqual(1);
  });

  it('blocks submit and shows a validation message when a deposit is submitted with no amount', async () => {
    const user = userEvent.setup();
    render(<ConfirmPaymentModal {...defaultProps} />);
    // "Tipo de cobro" is the first combobox; "Método de pago" is the second.
    const [chargeType] = screen.getAllByRole('combobox');
    if (!chargeType) throw new Error('charge type combobox not found');
    await user.click(chargeType);
    await user.click(await screen.findByRole('option', { name: /seña/i }));
    await user.click(screen.getByRole('button', { name: /confirmar pago/i }));
    expect(await screen.findByText(/el monto debe ser mayor a 0/i)).toBeInTheDocument();
    expect(defaultProps.onConfirm).not.toHaveBeenCalled();
  });
});

// 02-bookings-clients.md M3: the old `useConfirmPaymentForm` set
// `defaultValues: { amount: 0 }` and only corrected it to `remaining` inside
// a `queueMicrotask` (to dodge `react-hooks/set-state-in-effect`) — so the
// hidden `amount` field briefly held a stale `0` right after mount, before
// that microtask ran. Keying the body on `booking?.id` and computing
// `defaultValues` from `booking` up front removes that window.
describe('ConfirmPaymentModal — no stale-0 frame on the hidden amount field', () => {
  it('has the real remaining amount on the hidden field synchronously', () => {
    render(<ConfirmPaymentModal open onClose={vi.fn()} onConfirm={vi.fn()} booking={baseBooking} isLoading={false} />);
    const amountInput = document.querySelector('input[name="amount"]');
    expect(amountInput).toHaveValue(String(baseBooking.price));
  });
});
