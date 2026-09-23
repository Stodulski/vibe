import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import ProductDetailPage from './ProductDetailPage';
import { useSelectedComplex } from '@/features/complex/hooks/useSelectedComplex';
import { useProduct } from '../hooks/useProduct';
import { useProductStockMovements } from '../hooks/useProductStockMovements';
import { makeProduct } from '@/test/factories';

vi.mock('@/features/complex/hooks/useSelectedComplex', () => ({ useSelectedComplex: vi.fn() }));
vi.mock('../hooks/useProduct', () => ({ useProduct: vi.fn() }));
vi.mock('../hooks/useProductStockMovements', () => ({ useProductStockMovements: vi.fn() }));

function renderAt(productId: string) {
  // `ProductFormDialog`/`DeactivateProductDialog` are always mounted (closed)
  // alongside the header, same "needs a real QueryClient, not itself under
  // test here" shape as `ProductsPage.test.tsx`.
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[`/cash/products/${productId}`]}>
        <Routes>
          <Route path="/cash/products/:productId" element={<ProductDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('ProductDetailPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSelectedComplex).mockReturnValue({
      complex: { id: 'c1' },
      selectedComplexId: 'c1',
    } as unknown as ReturnType<typeof useSelectedComplex>);
    vi.mocked(useProductStockMovements).mockReturnValue({
      data: { pages: [{ stock_movements: [], metadata: { has_more: false } }] },
      isLoading: false,
      isError: false,
      isFetchNextPageError: false,
      refetch: vi.fn(),
      hasNextPage: false,
      fetchNextPage: vi.fn(),
      isFetchingNextPage: false,
    } as unknown as ReturnType<typeof useProductStockMovements>);
  });

  it('shows the product header and the stock-movement history section', () => {
    vi.mocked(useProduct).mockReturnValue({
      data: { product: makeProduct({ id: 'p1', name: 'Agua mineral', stock_on_hand: 12 }) },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useProduct>);

    renderAt('p1');

    expect(screen.getByText('Agua mineral')).toBeInTheDocument();
    expect(screen.getByText('Movimientos de stock')).toBeInTheDocument();
    expect(screen.getByText('Todavía no hay movimientos de stock')).toBeInTheDocument();
  });

  it('shows the full-screen load error when there is nothing cached', () => {
    vi.mocked(useProduct).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useProduct>);

    renderAt('p1');

    expect(screen.getByText('No pudimos cargar el producto')).toBeInTheDocument();
  });

  it('keeps rendering cached data with a stale notice instead of wiping the view on a background refetch failure', () => {
    vi.mocked(useProduct).mockReturnValue({
      data: { product: makeProduct({ id: 'p1', name: 'Agua mineral' }) },
      isLoading: false,
      isError: true,
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useProduct>);

    renderAt('p1');

    expect(screen.getByText('Agua mineral')).toBeInTheDocument();
    expect(screen.getByText('No pudimos actualizar. Mostrando los últimos datos guardados.')).toBeInTheDocument();
  });
});
