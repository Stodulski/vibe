import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import { makeSale, makeSaleBookingPayment, makeSaleItem } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { formatPrice } from '@/shared/lib/utils';
import { SaleConfirmationDialog, type SaleResult } from './SaleConfirmationDialog';

const t = ES_AR;

describe('SaleConfirmationDialog — mixed checkout', () => {
  it('lists the products and the booking balances, and totals both', () => {
    const result: SaleResult = {
      method: 'transfer',
      sale: makeSale({
        total: 300000,
        items: [makeSaleItem({ product_name: 'Agua 500ml', quantity: 2, line_total: 300000 })],
      }),
      bookingPayments: [
        makeSaleBookingPayment({
          court_name: 'Cancha 2',
          start_time: '19:00',
          client_name: 'Juan Pérez',
          amount: 350000,
        }),
      ],
      stockWarnings: [],
    };
    render(<SaleConfirmationDialog result={result} onClose={vi.fn()} />);

    const dialog = screen.getByRole('dialog');
    expect(within(dialog).getByText('2× Agua 500ml')).toBeInTheDocument();
    expect(within(dialog).getByText('Saldo turno · Cancha 2 19:00 · Juan Pérez')).toBeInTheDocument();
    expect(within(dialog).getByText(formatPrice(650000))).toBeInTheDocument();
    expect(within(dialog).getByText(t.bookings.paymentMethods.transfer)).toBeInTheDocument();
  });
});

describe('SaleConfirmationDialog — booking-only checkout', () => {
  it('shows the balances with no product lines when the checkout had no sale', () => {
    const result: SaleResult = {
      method: 'cash',
      sale: null,
      bookingPayments: [
        makeSaleBookingPayment({ booking_id: 'b1', amount: 200000 }),
        makeSaleBookingPayment({ booking_id: 'b2', payment_id: 'pay2', amount: 100000 }),
      ],
      stockWarnings: [],
    };
    render(<SaleConfirmationDialog result={result} onClose={vi.fn()} />);

    const dialog = screen.getByRole('dialog');
    expect(within(dialog).queryByText(/× /)).not.toBeInTheDocument();
    expect(within(dialog).getByText(formatPrice(300000))).toBeInTheDocument();
  });
});
