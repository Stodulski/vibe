import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { makeProduct } from '@/test/factories';
import { ProductFormDialog } from './ProductFormDialog';
import { useCreateProduct } from '../hooks/useCreateProduct';
import { useUpdateProduct } from '../hooks/useUpdateProduct';

vi.mock('../hooks/useCreateProduct', () => ({ useCreateProduct: vi.fn() }));
vi.mock('../hooks/useUpdateProduct', () => ({ useUpdateProduct: vi.fn() }));

describe('ProductFormDialog — create', () => {
  const createMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useCreateProduct).mockReturnValue({ mutate: createMutate, isPending: false } as unknown as ReturnType<
      typeof useCreateProduct
    >);
    vi.mocked(useUpdateProduct).mockReturnValue({ mutate: vi.fn(), isPending: false } as unknown as ReturnType<
      typeof useUpdateProduct
    >);
  });

  it('sends whole-pesos price in centavos and omits a blank category', async () => {
    const user = userEvent.setup();
    render(<ProductFormDialog open onClose={vi.fn()} complexId="c1" existingCategories={[]} />);

    await user.type(screen.getByLabelText('Nombre'), 'Agua mineral');
    await user.type(screen.getByLabelText('Precio'), '1500');
    await user.click(screen.getByRole('button', { name: 'Guardar' }));

    await waitFor(() => {
      expect(createMutate).toHaveBeenCalledWith(
        {
          name: 'Agua mineral',
          category: undefined,
          price: 150000,
          tracks_stock: true,
          low_stock_threshold: undefined,
        },
        expect.anything(),
      );
    });
  });

  it('has no threshold-locked helper text for a brand-new product', () => {
    render(<ProductFormDialog open onClose={vi.fn()} complexId="c1" existingCategories={[]} />);
    expect(screen.queryByText(/Ya tiene un aviso configurado/)).not.toBeInTheDocument();
  });
});

describe('ProductFormDialog — edit', () => {
  const updateMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useUpdateProduct).mockReturnValue({ mutate: updateMutate, isPending: false } as unknown as ReturnType<
      typeof useUpdateProduct
    >);
    vi.mocked(useCreateProduct).mockReturnValue({ mutate: vi.fn(), isPending: false } as unknown as ReturnType<
      typeof useCreateProduct
    >);
  });

  it('shows the locked-threshold helper text once a product already has one, and refuses to clear it', async () => {
    const user = userEvent.setup();
    const product = makeProduct({ id: 'p1', low_stock_threshold: 5, version: 3 });
    render(<ProductFormDialog open onClose={vi.fn()} complexId="c1" product={product} existingCategories={[]} />);

    expect(screen.getByText(/Ya tiene un aviso configurado/)).toBeInTheDocument();

    await user.clear(screen.getByLabelText('Avisar con stock bajo en'));
    await user.click(screen.getByRole('button', { name: 'Guardar' }));

    expect(await screen.findByText('No se puede borrar una vez definido; ingresá otro valor.')).toBeInTheDocument();
    expect(updateMutate).not.toHaveBeenCalled();
  });

  it('sends an explicit empty category (not omitted) so PATCH clears it, and the product version', async () => {
    const user = userEvent.setup();
    const product = makeProduct({ id: 'p1', category: 'Bebidas', low_stock_threshold: null, version: 3 });
    render(<ProductFormDialog open onClose={vi.fn()} complexId="c1" product={product} existingCategories={[]} />);

    await user.clear(screen.getByLabelText('Categoría'));
    await user.click(screen.getByRole('button', { name: 'Guardar' }));

    await waitFor(() => {
      expect(updateMutate).toHaveBeenCalled();
    });
    const [call] = updateMutate.mock.calls as [{ productId: string; data: { version: number; category: string } }][];
    expect(call?.[0]).toMatchObject({ productId: 'p1', data: { version: 3, category: '' } });
  });
});
