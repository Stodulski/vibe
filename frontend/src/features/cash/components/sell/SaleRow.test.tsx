import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { makeSale, makeSaleItem } from '@/test/factories';
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

    expect(screen.getByText('2× Agua 500ml, 1× Pelotas')).toBeInTheDocument();
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
