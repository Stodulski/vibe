import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ProductCategoryChips } from './ProductCategoryChips';

const categories = ['Accesorios', 'Bebidas', 'Pelotas'];

describe('ProductCategoryChips', () => {
  // Inside a dialog at 320px a sideways slider cut its last chip mid-word ("Ac…"):
  // every chip is shown whole, wrapping onto the next line instead.
  it('wraps the chips instead of scrolling them sideways', () => {
    render(<ProductCategoryChips categories={categories} value="" onSelect={vi.fn()} ariaLabel="Categorías" />);
    const group = screen.getByRole('group', { name: 'Categorías' });
    expect(group).toHaveClass('flex-wrap');
    expect(group).not.toHaveClass('overflow-x-auto');
  });

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
