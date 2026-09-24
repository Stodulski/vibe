import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen } from '@testing-library/react';
import { LowStockAlert } from './LowStockAlert';
import { renderWithProviders } from '@/test/test-utils';
import { useProducts } from '@/shared/hooks/useProducts';
import { makeProduct } from '@/test/factories';
import type { Product } from '@/shared/types/api.types';

vi.mock('@/shared/hooks/useProducts', () => ({ useProducts: vi.fn() }));

function mockLoading() {
  vi.mocked(useProducts).mockReturnValue({
    data: undefined,
    isLoading: true,
    isError: false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useProducts>);
}

function mockProducts(products: Product[], overrides: { isError?: boolean } = {}) {
  vi.mocked(useProducts).mockReturnValue({
    data: { products },
    isLoading: false,
    isError: overrides.isError ?? false,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useProducts>);
}

function mockRealErrorNoData() {
  vi.mocked(useProducts).mockReturnValue({
    data: undefined,
    isLoading: false,
    isError: true,
    refetch: vi.fn(),
  } as unknown as ReturnType<typeof useProducts>);
}

describe('LowStockAlert', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('shows a loading skeleton while the products query is in flight', () => {
    mockLoading();
    renderWithProviders(<LowStockAlert complexId="c1" />);
    expect(screen.getByRole('status')).toHaveAttribute('aria-busy', 'true');
  });

  it('renders nothing when there are no low-stock products', () => {
    mockProducts([makeProduct({ low_stock: false, needs_stock_review: false })]);
    const { container } = renderWithProviders(<LowStockAlert complexId="c1" />);
    expect(container).toBeEmptyDOMElement();
  });

  it('renders nothing for an untracked product even if flagged', () => {
    mockProducts([makeProduct({ tracks_stock: false, low_stock: true })]);
    const { container } = renderWithProviders(<LowStockAlert complexId="c1" />);
    expect(container).toBeEmptyDOMElement();
  });

  it('lists flagged products, negative stock first then lowest stock, capped at 5, with a link to the products screen', () => {
    const products = [
      makeProduct({ id: 'p1', name: 'Bajo positivo', stock_on_hand: 2, low_stock: true }),
      makeProduct({ id: 'p2', name: 'Negativo', stock_on_hand: -3, needs_stock_review: true }),
      makeProduct({ id: 'p3', name: 'OK', stock_on_hand: 50, low_stock: false, needs_stock_review: false }),
      makeProduct({ id: 'p4', name: 'Bajo 1', stock_on_hand: 1, low_stock: true }),
      makeProduct({ id: 'p5', name: 'Bajo 3', stock_on_hand: 3, low_stock: true }),
      makeProduct({ id: 'p6', name: 'Bajo 4', stock_on_hand: 4, low_stock: true }),
      makeProduct({ id: 'p7', name: 'Bajo 5', stock_on_hand: 5, low_stock: true }),
    ];
    mockProducts(products);
    renderWithProviders(<LowStockAlert complexId="c1" />);

    const items = screen.getAllByRole('listitem');
    expect(items).toHaveLength(5);
    // Negative stock first, then ascending: Negativo(-3), Bajo1(1), Bajo positivo(2), Bajo3(3), Bajo4(4) — Bajo5(5) is cut by the 5-item cap.
    expect(items.map((li) => li.textContent)).toEqual([
      'Negativo-3',
      'Bajo 11',
      'Bajo positivo2',
      'Bajo 33',
      'Bajo 44',
    ]);
    expect(screen.queryByText('Bajo 5')).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Ver productos' })).toHaveAttribute('href', '/cash/products');
  });

  it('shows a compact retry with no full-page takeover when there is no cached data', () => {
    mockRealErrorNoData();
    renderWithProviders(<LowStockAlert complexId="c1" />);
    expect(screen.getByRole('button', { name: 'Reintentar' })).toBeInTheDocument();
  });

  it('keeps rendering cached low-stock data with a non-blocking notice on a background refetch failure', () => {
    mockProducts([makeProduct({ id: 'p1', name: 'Bajo', stock_on_hand: 1, low_stock: true })], { isError: true });
    renderWithProviders(<LowStockAlert complexId="c1" />);
    expect(screen.getByText('Bajo')).toBeInTheDocument();
    expect(screen.getByRole('status')).toBeInTheDocument();
  });
});
