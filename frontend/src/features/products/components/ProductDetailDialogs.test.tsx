import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { makeProduct } from '@/test/factories';
import { ProductDetailDialogs } from './ProductDetailDialogs';
import { useProducts } from '../hooks/useProducts';
import { useCreateProduct } from '../hooks/useCreateProduct';
import { useUpdateProduct } from '../hooks/useUpdateProduct';

vi.mock('../hooks/useProducts', () => ({ useProducts: vi.fn() }));
vi.mock('../hooks/useCreateProduct', () => ({ useCreateProduct: vi.fn() }));
vi.mock('../hooks/useUpdateProduct', () => ({ useUpdateProduct: vi.fn() }));

function renderDialogs(product = makeProduct({ id: 'p1', category: 'Bebidas' })) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const dialogs = {
    editOpen: true,
    setEditOpen: vi.fn(),
    restockOpen: false,
    setRestockOpen: vi.fn(),
    adjustOpen: false,
    setAdjustOpen: vi.fn(),
    toggleOpen: false,
    setToggleOpen: vi.fn(),
  };

  return render(
    <QueryClientProvider client={queryClient}>
      <ProductDetailDialogs product={product} complexId="c1" dialogs={dialogs} />
    </QueryClientProvider>,
  );
}

describe('ProductDetailDialogs', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useCreateProduct).mockReturnValue({ mutate: vi.fn(), isPending: false } as unknown as ReturnType<
      typeof useCreateProduct
    >);
    vi.mocked(useUpdateProduct).mockReturnValue({ mutate: vi.fn(), isPending: false } as unknown as ReturnType<
      typeof useUpdateProduct
    >);
  });

  it('offers every catalog category, not just the product being edited', () => {
    vi.mocked(useProducts).mockReturnValue({
      data: {
        products: [makeProduct({ id: 'p1', category: 'Bebidas' }), makeProduct({ id: 'p2', category: 'Snacks' })],
      },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useProducts>);

    renderDialogs(makeProduct({ id: 'p1', category: 'Bebidas' }));

    expect(screen.getByRole('button', { name: 'Snacks' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Bebidas' })).toBeInTheDocument();
  });
});
