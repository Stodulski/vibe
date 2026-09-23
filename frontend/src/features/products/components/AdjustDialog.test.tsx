import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { makeProduct } from '@/test/factories';
import { AdjustDialog } from './AdjustDialog';
import { useAdjustProduct } from '../hooks/useAdjustProduct';

vi.mock('../hooks/useAdjustProduct', () => ({ useAdjustProduct: vi.fn() }));

describe('AdjustDialog — computed difference', () => {
  const mutate = vi.fn();
  const product = makeProduct({ id: 'p1', stock_on_hand: 20 });

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useAdjustProduct).mockReturnValue({ mutate, isPending: false } as unknown as ReturnType<
      typeof useAdjustProduct
    >);
  });

  it('shows the current stock and no difference before anything is typed', () => {
    render(<AdjustDialog open onClose={vi.fn()} complexId="c1" product={product} />);
    expect(screen.getByTestId('adjust-current-stock')).toHaveTextContent('Stock actual: 20');
    expect(screen.queryByTestId('adjust-difference')).not.toBeInTheDocument();
  });

  it('shows a positive difference and enables submit when counted is higher than current stock', async () => {
    const user = userEvent.setup();
    render(<AdjustDialog open onClose={vi.fn()} complexId="c1" product={product} />);

    await user.type(screen.getByLabelText('Cantidad contada'), '25');

    expect(screen.getByTestId('adjust-difference')).toHaveTextContent('+5');
    expect(screen.getByRole('button', { name: 'Ajustar' })).toBeEnabled();
  });

  it('shows a negative difference when counted is lower than current stock', async () => {
    const user = userEvent.setup();
    render(<AdjustDialog open onClose={vi.fn()} complexId="c1" product={product} />);

    await user.type(screen.getByLabelText('Cantidad contada'), '15');

    expect(screen.getByTestId('adjust-difference')).toHaveTextContent('-5');
  });

  it('disables submit when the counted quantity equals current stock (0 difference is not a real adjustment)', async () => {
    const user = userEvent.setup();
    render(<AdjustDialog open onClose={vi.fn()} complexId="c1" product={product} />);

    await user.type(screen.getByLabelText('Cantidad contada'), '20');

    expect(screen.getByTestId('adjust-difference')).toHaveTextContent('0');
    expect(screen.getByRole('button', { name: 'Ajustar' })).toBeDisabled();
  });

  it('sends the signed difference (not the counted value) as quantity on submit', async () => {
    const user = userEvent.setup();
    render(<AdjustDialog open onClose={vi.fn()} complexId="c1" product={product} />);

    await user.type(screen.getByLabelText('Cantidad contada'), '25');
    await user.click(screen.getByRole('button', { name: 'Ajustar' }));

    await waitFor(() => {
      expect(mutate).toHaveBeenCalledWith({ quantity: 5, reason: 'breakage', note: undefined }, expect.anything());
    });
  });
});
