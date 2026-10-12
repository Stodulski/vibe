import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { makeSale, makeSaleBookingPayment, makeSaleItem } from '@/test/factories';
import { SaleRow } from './SaleRow';

describe('SaleRow', () => {
  it('shows the items summary, total and method', () => {
    const sale = makeSale({
      method: 'transfer',
      total: 300000,
      items: [
        makeSaleItem({ product_name: 'Agua 500ml', quantity: 2 }),
        makeSaleItem({ product_name: 'Pelotas', quantity: 1 }),
      ],
    });
    render(<SaleRow sale={sale} onVoid={vi.fn()} />);

    const items = screen.getAllByRole('listitem').map((li) => li.textContent);
    expect(items).toEqual(['2× Agua 500ml', '1× Pelotas']);
    expect(screen.getByText('$3.000')).toBeInTheDocument();
    expect(screen.getByText(/Transferencia/)).toBeInTheDocument();
  });

  it('shows Anular for an active sale', () => {
    render(<SaleRow sale={makeSale({ voided_at: null })} onVoid={vi.fn()} />);
    expect(screen.getByRole('button', { name: 'Anular' })).toBeInTheDocument();
  });

  it('hides Anular and shows Anulada for a voided sale', () => {
    render(<SaleRow sale={makeSale({ voided_at: '2026-01-01T16:00:00Z' })} onVoid={vi.fn()} />);
    expect(screen.queryByRole('button', { name: 'Anular' })).not.toBeInTheDocument();
    expect(screen.getByText('Anulada')).toBeInTheDocument();
  });

  it('calls onVoid when Anular is clicked', () => {
    const onVoid = vi.fn();
    render(<SaleRow sale={makeSale({ voided_at: null })} onVoid={onVoid} />);
    screen.getByRole('button', { name: 'Anular' }).click();
    expect(onVoid).toHaveBeenCalledTimes(1);
  });
});

describe('SaleRow — booking balances', () => {
  it('lists the booking balances collected in the same checkout, with the booking summary', () => {
    const sale = makeSale({
      booking_payments: [
        makeSaleBookingPayment({
          court_name: 'Cancha 2',
          start_time: '19:00',
          client_name: 'Juan Pérez',
          amount: 350000,
        }),
      ],
    });
    render(<SaleRow sale={sale} onVoid={vi.fn()} />);

    expect(screen.getByText('Saldo turno · Cancha 2 19:00 · Juan Pérez · $3.500')).toBeInTheDocument();
  });

  it('disables Anular for a sale with booking balances and explains why', async () => {
    const user = userEvent.setup();
    const onVoid = vi.fn();
    const sale = makeSale({ voided_at: null, booking_payments: [makeSaleBookingPayment()] });
    render(<SaleRow sale={sale} onVoid={onVoid} />);

    const anular = screen.getByRole('button', { name: 'Anular' });
    expect(anular).toBeDisabled();
    expect(screen.getByText('Esta venta incluye el saldo de un turno y no se puede anular.')).toBeInTheDocument();

    await user.click(anular);
    expect(onVoid).not.toHaveBeenCalled();
  });

  it('keeps Anular for a sale with no booking balances', () => {
    render(<SaleRow sale={makeSale({ voided_at: null, booking_payments: [] })} onVoid={vi.fn()} />);

    expect(screen.getByRole('button', { name: 'Anular' })).toBeEnabled();
    expect(screen.queryByText('Esta venta incluye el saldo de un turno y no se puede anular.')).not.toBeInTheDocument();
  });
});
