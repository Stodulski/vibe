import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ProductCategoryField } from './ProductCategoryField';

describe('ProductCategoryField', () => {
  it('renders existing and starter categories deduplicated', () => {
    render(<ProductCategoryField value="" onChange={vi.fn()} suggestions={['Bebidas', 'Cafetería']} />);

    // "Bebidas" comes from the catalog, not duplicated by the starter list.
    expect(screen.getAllByRole('button', { name: 'Bebidas' })).toHaveLength(1);
    expect(screen.getByRole('button', { name: 'Cafetería' })).toBeInTheDocument();
    // Starter categories not already in the catalog still show up.
    expect(screen.getByRole('button', { name: 'Snacks' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Comida' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Accesorios' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Alquiler' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Indumentaria' })).toBeInTheDocument();
  });

  it('fills the field when a chip is tapped, and clears it when tapped again', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const { rerender } = render(<ProductCategoryField value="" onChange={onChange} suggestions={['Bebidas']} />);

    await user.click(screen.getByRole('button', { name: 'Bebidas' }));
    expect(onChange).toHaveBeenLastCalledWith('Bebidas');

    rerender(<ProductCategoryField value="Bebidas" onChange={onChange} suggestions={['Bebidas']} />);
    const chip = screen.getByRole('button', { name: 'Bebidas' });
    expect(chip).toHaveAttribute('aria-pressed', 'true');

    await user.click(chip);
    expect(onChange).toHaveBeenLastCalledWith('');
  });

  it('has no chip pressed when the value matches nothing', () => {
    render(<ProductCategoryField value="Fiambrería" onChange={vi.fn()} suggestions={['Bebidas']} />);

    expect(screen.getByRole('button', { name: 'Bebidas' })).toHaveAttribute('aria-pressed', 'false');
  });

  it('allows typing a brand-new category', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<ProductCategoryField value="" onChange={onChange} suggestions={[]} />);

    await user.type(screen.getByLabelText('Categoría'), 'Fiambrería');

    // A controlled input with a fixed `value=""` re-renders with the same
    // empty value after every keystroke, so each call reports the single
    // new character rather than an accumulating string — what matters here
    // is that free typing still reaches `onChange` for every keystroke.
    expect(onChange).toHaveBeenCalledTimes('Fiambrería'.length);
    expect(onChange).toHaveBeenLastCalledWith('a');
  });
});
