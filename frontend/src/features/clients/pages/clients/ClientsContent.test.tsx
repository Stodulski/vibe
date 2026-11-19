import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ClientsContent } from './ClientsContent';
import { makeClient } from '@/test/factories';

const baseProps = {
  isLoading: false,
  isError: false,
  onRetry: vi.fn(),
  clients: [],
  hasNextPage: false,
  isFetchingNextPage: false,
  sentinelRef: { current: null },
  onSelectClient: vi.fn(),
  onBlockClient: vi.fn(),
};

describe('ClientsContent', () => {
  it('shows the empty state when there are no clients and no error', () => {
    render(<ClientsContent {...baseProps} />);
    expect(screen.getByText('Todavía no hay clientes registrados')).toBeInTheDocument();
  });

  // The grid grows as you scroll and shrinks as you filter, with nothing to
  // announce either (A11Y-07).
  it('announces how many clients the grid is showing', () => {
    const clients = [makeClient({ id: 'c1' }), makeClient({ id: 'c2' })];
    render(<ClientsContent {...baseProps} clients={clients} />);

    const region = screen.getByText('2 resultados');
    expect(region).toHaveAttribute('aria-live', 'polite');
  });

  // The case that matters most when filtering, and the one an announcer
  // living inside the populated branch could never reach.
  it('announces "0 resultados" alongside the empty state', () => {
    render(<ClientsContent {...baseProps} />);

    expect(screen.getByText('Todavía no hay clientes registrados')).toBeInTheDocument();
    expect(screen.getByText('0 resultados')).toHaveAttribute('aria-live', 'polite');
  });

  // Mounted before the rows exist, so the browser has a region to announce
  // into when the count arrives — but silent until the query settles, or it
  // would announce a zero the server never returned.
  it('keeps the live region mounted and silent while loading and on error', () => {
    const { container, rerender } = render(<ClientsContent {...baseProps} isLoading={true} />);

    const region = container.querySelector('[aria-live="polite"]');
    expect(region).toBeInTheDocument();
    expect(region).toHaveTextContent('');

    rerender(<ClientsContent {...baseProps} isError={true} />);
    expect(container.querySelector('[aria-live="polite"]')).toHaveTextContent('');

    rerender(<ClientsContent {...baseProps} clients={[makeClient({ id: 'c1' })]} />);
    expect(container.querySelector('[aria-live="polite"]')).toHaveTextContent('1 resultado');
  });

  it('shows the error state instead of the empty state when the query fails', () => {
    render(<ClientsContent {...baseProps} isError={true} />);
    expect(screen.getByText('No pudimos cargar los datos.')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Reintentar' })).toBeInTheDocument();
    expect(screen.queryByText('Todavía no hay clientes registrados')).not.toBeInTheDocument();
  });

  it('calls onRetry when the retry button is clicked', async () => {
    const onRetry = vi.fn();
    render(<ClientsContent {...baseProps} isError={true} onRetry={onRetry} />);
    await userEvent.click(screen.getByRole('button', { name: 'Reintentar' }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });
});
