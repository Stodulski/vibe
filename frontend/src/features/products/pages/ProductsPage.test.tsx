import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import ProductsPage from './ProductsPage';
import { useSelectedComplex } from '@/features/complex/hooks/useSelectedComplex';
import { useProducts } from '../hooks/useProducts';
import { makeProduct } from '@/test/factories';

vi.mock('@/features/complex/hooks/useSelectedComplex', () => ({ useSelectedComplex: vi.fn() }));
vi.mock('../hooks/useProducts', () => ({ useProducts: vi.fn() }));

function mockProductsList(products: ReturnType<typeof makeProduct>[]) {
  vi.mocked(useProducts).mockReturnValue({
    data: { products },
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useProducts>);
}

function renderPage() {
  // `DeactivateProductDialog` (always mounted regardless of target, same
  // shape as `CashPage.test.tsx`'s `VoidMovementDialog` note) calls
  // `useUpdateProduct`, which reaches for a real `useQueryClient()` — not
  // itself mocked here, since it's exercised by its own dedicated tests, but
  // it still needs a provider to mount at all.
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={['/cash/products']}>
        <ProductsPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe('ProductsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSelectedComplex).mockReturnValue({
      complex: { id: 'c1' },
      selectedComplexId: 'c1',
    } as unknown as ReturnType<typeof useSelectedComplex>);
  });

  it('renders the section tabs and the catalog', () => {
    mockProductsList([makeProduct({ id: 'p1', name: 'Agua mineral' })]);
    renderPage();

    expect(screen.getByRole('link', { name: 'Turno' })).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Productos' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByText('Agua mineral')).toBeInTheDocument();
  });

  it('shows the empty state with a call to action when the catalog is empty', () => {
    mockProductsList([]);
    renderPage();

    expect(screen.getByText('Todavía no cargaste productos')).toBeInTheDocument();
    // Two buttons share the label: the header's own primary action and the
    // empty state's (rendered with `actionVariant="outline"` precisely
    // because it duplicates that primary action — `EmptyState`'s own doc
    // comment).
    expect(screen.getAllByRole('button', { name: 'Nuevo producto' })).toHaveLength(2);
  });

  it('filters the visible list by name/category as the owner types, client-side', async () => {
    const user = userEvent.setup();
    mockProductsList([
      makeProduct({ id: 'p1', name: 'Agua mineral', category: 'Bebidas' }),
      makeProduct({ id: 'p2', name: 'Paleta de pádel', category: 'Accesorios' }),
    ]);
    renderPage();

    await user.type(screen.getByLabelText('Buscar por nombre o categoría...'), 'pádel');

    expect(screen.getByText('Paleta de pádel')).toBeInTheDocument();
    expect(screen.queryByText('Agua mineral')).not.toBeInTheDocument();
  });

  it('switches to the Inactivos filter and refetches with active=false', async () => {
    const user = userEvent.setup();
    mockProductsList([makeProduct({ id: 'p1', name: 'Agua mineral' })]);
    renderPage();

    await user.click(screen.getByRole('button', { name: 'Inactivos' }));

    expect(useProducts).toHaveBeenLastCalledWith('c1', false);
  });
});
