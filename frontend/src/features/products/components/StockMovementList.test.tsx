import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { StockMovementList } from './StockMovementList';
import { useProductStockMovements } from '../hooks/useProductStockMovements';
import { makeStockMovement } from '@/test/factories';
import type { StockMovement } from '@/shared/types/api.types';

vi.mock('../hooks/useProductStockMovements', () => ({ useProductStockMovements: vi.fn() }));

/** Shared defaults for `useProductStockMovements`'s mocked return, overridden per test. */
function mockMovements(overrides: Record<string, unknown>) {
  vi.mocked(useProductStockMovements).mockReturnValue({
    data: undefined,
    isLoading: false,
    isError: false,
    isFetchNextPageError: false,
    refetch: vi.fn(),
    hasNextPage: false,
    fetchNextPage: vi.fn(),
    isFetchingNextPage: false,
    ...overrides,
  } as unknown as ReturnType<typeof useProductStockMovements>);
}

function pageWith(movements: StockMovement[], metadata: { has_more: boolean; next_cursor?: string }) {
  return { data: { pages: [{ stock_movements: movements, metadata }] } };
}

describe('StockMovementList', () => {
  it('shows the empty state when there is no history', () => {
    mockMovements(pageWith([], { has_more: false }));

    render(<StockMovementList complexId="c1" productId="p1" />);
    expect(screen.getByText('Todavía no hay movimientos de stock')).toBeInTheDocument();
  });

  it('keeps every already-loaded page on screen and retries the next page on click when it fails', async () => {
    const user = userEvent.setup();
    const fetchNextPage = vi.fn();
    mockMovements({
      ...pageWith([makeStockMovement({ id: 'sm1', kind: 'restock', quantity: 10 })], {
        has_more: true,
        next_cursor: 'c2',
      }),
      isFetchNextPageError: true,
      hasNextPage: true,
      fetchNextPage,
    });

    render(<StockMovementList complexId="c1" productId="p1" />);

    // The already-loaded page's row is still there — a next-page failure
    // never replaces the whole list with the full-screen error.
    expect(screen.getByText('Reposición')).toBeInTheDocument();
    expect(screen.getByText('+10')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Reintentar' }));
    expect(fetchNextPage).toHaveBeenCalledTimes(1);
  });

  it('shows the stale-data notice (not the full-screen error) when a background refetch fails with rows already cached', async () => {
    const user = userEvent.setup();
    const refetch = vi.fn();
    mockMovements({
      ...pageWith([makeStockMovement({ id: 'sm1', kind: 'restock', quantity: 10 })], { has_more: false }),
      isError: true,
      refetch,
    });

    render(<StockMovementList complexId="c1" productId="p1" />);

    // The already-loaded row is still shown, not the full-screen error.
    expect(screen.getByText('Reposición')).toBeInTheDocument();
    expect(screen.getByRole('status')).toHaveTextContent('No pudimos actualizar');

    await user.click(screen.getByRole('button', { name: 'Reintentar' }));
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it('shows the full-screen error only when nothing is cached at all', () => {
    mockMovements({ isError: true });

    render(<StockMovementList complexId="c1" productId="p1" />);
    expect(screen.getByText('No pudimos cargar los datos.')).toBeInTheDocument();
  });
});
