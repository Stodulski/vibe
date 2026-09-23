import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { StockMovementList } from './StockMovementList';
import { useProductStockMovements } from '../hooks/useProductStockMovements';
import { makeStockMovement } from '@/test/factories';

vi.mock('../hooks/useProductStockMovements', () => ({ useProductStockMovements: vi.fn() }));

describe('StockMovementList', () => {
  it('shows the empty state when there is no history', () => {
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

    render(<StockMovementList complexId="c1" productId="p1" />);
    expect(screen.getByText('Todavía no hay movimientos de stock')).toBeInTheDocument();
  });

  it('keeps every already-loaded page on screen and shows a retry footer when the next page fails', () => {
    const fetchNextPage = vi.fn();
    vi.mocked(useProductStockMovements).mockReturnValue({
      data: {
        pages: [
          {
            stock_movements: [makeStockMovement({ id: 'sm1', kind: 'restock', quantity: 10 })],
            metadata: { has_more: true, next_cursor: 'c2' },
          },
        ],
      },
      isLoading: false,
      isError: false,
      isFetchNextPageError: true,
      refetch: vi.fn(),
      hasNextPage: true,
      fetchNextPage,
      isFetchingNextPage: false,
    } as unknown as ReturnType<typeof useProductStockMovements>);

    render(<StockMovementList complexId="c1" productId="p1" />);

    // The already-loaded page's row is still there — a next-page failure
    // never replaces the whole list with the full-screen error.
    expect(screen.getByText('Reposición')).toBeInTheDocument();
    expect(screen.getByText('+10')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Reintentar' })).toBeInTheDocument();
  });

  it('shows the full-screen error only when nothing is cached at all', () => {
    vi.mocked(useProductStockMovements).mockReturnValue({
      data: undefined,
      isLoading: false,
      isError: true,
      isFetchNextPageError: false,
      refetch: vi.fn(),
      hasNextPage: false,
      fetchNextPage: vi.fn(),
      isFetchingNextPage: false,
    } as unknown as ReturnType<typeof useProductStockMovements>);

    render(<StockMovementList complexId="c1" productId="p1" />);
    expect(screen.getByText('No pudimos cargar los datos.')).toBeInTheDocument();
  });
});
