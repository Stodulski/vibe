import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ProductCategoryChips } from './ProductCategoryChips';

const categories = ['Accesorios', 'Bebidas', 'Pelotas'];

describe('ProductCategoryChips', () => {
  it('fills the field with the tapped chip and clears it when tapped again', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    const { rerender } = render(
      <ProductCategoryChips categories={categories} value="" onSelect={onSelect} ariaLabel="Categorías" />,
    );
    await user.click(screen.getByRole('button', { name: 'Bebidas' }));
    expect(onSelect).toHaveBeenLastCalledWith('Bebidas');

    rerender(
      <ProductCategoryChips categories={categories} value="bebidas" onSelect={onSelect} ariaLabel="Categorías" />,
    );
    await user.click(screen.getByRole('button', { name: 'Bebidas' }));
    expect(onSelect).toHaveBeenLastCalledWith('');
  });
});
