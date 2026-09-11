import { render, screen } from '@testing-library/react';
import { BlockClientModal } from './BlockClientModal';
import type { Client } from '@/shared/types/api.types';

const client = {
  id: 'c1',
  complex_id: 'x1',
  first_name: 'Sofia',
  last_name: 'Perez',
  phone: '+5491140000002',
  is_blocked: false,
  total_bookings: 5,
  no_show_count: 0,
  cancelled_count: 0,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
} as unknown as Client;

describe('BlockClientModal', () => {
  const defaultProps = {
    open: true,
    onClose: vi.fn(),
    onConfirm: vi.fn(),
    isLoading: false,
  };

  it('names the client it is about to block', () => {
    render(<BlockClientModal {...defaultProps} client={client} />);
    expect(screen.getByText(/sofia perez/i)).toBeInTheDocument();
  });

  // The caller drives `open` off this same client and clears it to close, so
  // both props drop in one tick. Returning null there tore the dialog out of
  // the tree before it could fade, which is what this guards.
  it('keeps rendering the client after the caller clears it', () => {
    const { rerender } = render(<BlockClientModal {...defaultProps} client={client} />);

    rerender(<BlockClientModal {...defaultProps} client={null} />);

    expect(screen.getByText(/sofia perez/i)).toBeInTheDocument();
  });

  it('renders nothing before a client is ever handed in', () => {
    const { container } = render(<BlockClientModal {...defaultProps} open={false} client={null} />);
    expect(container).toBeEmptyDOMElement();
  });
});
