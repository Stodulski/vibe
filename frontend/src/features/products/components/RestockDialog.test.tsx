import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import userEvent from '@testing-library/user-event';
import { makeConsumedHttpError, makeProduct } from '@/test/factories';
import { RestockDialog } from './RestockDialog';
import { useRestockProduct } from '../hooks/useRestockProduct';
import { useCashSession } from '@/shared/hooks/useCashSession';
import type { Product } from '@/shared/types/api.types';

vi.mock('../hooks/useRestockProduct', () => ({ useRestockProduct: vi.fn() }));
vi.mock('@/shared/hooks/useCashSession', () => ({ useCashSession: vi.fn() }));

function renderDialog(product: Product) {
  return render(
    <MemoryRouter>
      <RestockDialog open onClose={vi.fn()} complexId="c1" product={product} />
    </MemoryRouter>,
  );
}

function mockTillOpen() {
  vi.mocked(useCashSession).mockReturnValue({ isClosed: false } as unknown as ReturnType<typeof useCashSession>);
}

function mockTillClosed() {
  vi.mocked(useCashSession).mockReturnValue({ isClosed: true } as unknown as ReturnType<typeof useCashSession>);
}

describe('RestockDialog — closed-till state', () => {
  const mutate = vi.fn();
  const product = makeProduct({ id: 'p1' });

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useRestockProduct).mockReturnValue({ mutate, isPending: false } as unknown as ReturnType<
      typeof useRestockProduct
    >);
  });

  it('shows the closed-till explanation and a link to Caja, and disables submit', () => {
    mockTillClosed();
    renderDialog(product);

    expect(screen.getByText(/Para reponer necesitás la caja abierta/)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Ir a Caja' })).toHaveAttribute('href', '/cash');
    expect(screen.getByRole('button', { name: 'Reponer' })).toBeDisabled();
  });

  it('shows no notice and an enabled submit when the till is open', () => {
    mockTillOpen();
    renderDialog(product);

    expect(screen.queryByText(/Para reponer necesitás la caja abierta/)).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Reponer' })).toBeEnabled();
  });
});

describe('RestockDialog — submit and error mapping', () => {
  const mutate = vi.fn();
  const product = makeProduct({ id: 'p1' });

  beforeEach(() => {
    vi.clearAllMocks();
    mockTillOpen();
    vi.mocked(useRestockProduct).mockReturnValue({ mutate, isPending: false } as unknown as ReturnType<
      typeof useRestockProduct
    >);
  });

  it('converts the typed pesos cost to centavos on submit', async () => {
    const user = userEvent.setup();
    renderDialog(product);

    await user.type(screen.getByLabelText('Cantidad'), '10');
    await user.type(screen.getByLabelText('Costo total'), '5000');
    await user.click(screen.getByRole('button', { name: 'Reponer' }));

    await waitFor(() => {
      expect(mutate).toHaveBeenCalledWith(
        { quantity: 10, total_cost: 500000, method: 'cash', note: undefined },
        expect.anything(),
      );
    });
  });

  it('maps a 422 field error from the server onto the quantity field', async () => {
    const user = userEvent.setup();
    mutate.mockImplementation((_vars: unknown, options: { onError: (error: unknown) => void }) => {
      void makeConsumedHttpError(422, {
        title: 'Validation Failed',
        errors: [{ field: 'quantity', message: 'demasiado grande' }],
      }).then((error) => {
        options.onError(error);
      });
    });
    renderDialog(product);

    await user.type(screen.getByLabelText('Cantidad'), '10');
    await user.type(screen.getByLabelText('Costo total'), '5000');
    await user.click(screen.getByRole('button', { name: 'Reponer' }));

    expect(await screen.findByText('demasiado grande')).toBeInTheDocument();
  });
});
