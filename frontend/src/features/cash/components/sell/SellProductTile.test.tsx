import { describe, it, expect, vi } from 'vitest';
import { renderWithProviders, screen } from '@/test/test-utils';
import { makeProduct } from '@/test/factories';
import { SellProductTile } from './SellProductTile';

describe('SellProductTile in-cart mark', () => {
  it('shows the count once the product is in the cart, keeping the button named after the product', () => {
    renderWithProviders(<SellProductTile product={makeProduct({ name: 'Agua' })} quantity={2} onTap={vi.fn()} />);

    expect(screen.getByText('2 en el carrito')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Agua' })).toBeInTheDocument();
  });

  it('shows no mark while the product is not in the cart', () => {
    renderWithProviders(<SellProductTile product={makeProduct({ name: 'Agua' })} quantity={0} onTap={vi.fn()} />);

    expect(screen.queryByText(/en el carrito/)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Agua' })).toBeInTheDocument();
  });
});
