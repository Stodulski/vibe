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
