import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { CancelBookingModal } from './CancelBookingModal';

const bookingA = {
  court_name: 'Cancha 1',
  date: '2026-03-15',
  starts_at: '2026-03-15T10:00:00-03:00',
  ends_at: '2026-03-15T11:30:00-03:00',
  client_name: 'Juan Perez',
};

const bookingB = { ...bookingA, court_name: 'Cancha 2', client_name: 'Ana Gomez' };

describe('CancelBookingModal', () => {
  const defaultProps = {
    open: true,
    onClose: vi.fn(),
    onConfirm: vi.fn(),
    isLoading: false,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders title when open', () => {
    render(<CancelBookingModal {...defaultProps} />);
    expect(screen.getByText(/cancelar reserva/i)).toBeInTheDocument();
  });

  it('renders confirmation description', () => {
    render(<CancelBookingModal {...defaultProps} />);
    expect(screen.getByText(/est.s seguro que quer.s cancelar/i)).toBeInTheDocument();
  });

  it('renders cancel reason textarea', () => {
    render(<CancelBookingModal {...defaultProps} />);
    expect(screen.getByPlaceholderText(/motivo de la cancelaci.n/i)).toBeInTheDocument();
  });

  it('calls onConfirm with undefined when no reason entered', async () => {
    const user = userEvent.setup();
    render(<CancelBookingModal {...defaultProps} />);
    await user.click(screen.getByRole('button', { name: /confirmar cancelaci.n/i }));
    expect(defaultProps.onConfirm).toHaveBeenCalledWith(undefined);
  });

  it('calls onConfirm with reason when entered', async () => {
    const user = userEvent.setup();
    render(<CancelBookingModal {...defaultProps} />);
    await user.type(screen.getByPlaceholderText(/motivo de la cancelaci.n/i), 'Cliente no puede asistir');
    await user.click(screen.getByRole('button', { name: /confirmar cancelaci.n/i }));
    expect(defaultProps.onConfirm).toHaveBeenCalledWith('Cliente no puede asistir');
  });

  it('disables buttons when loading', () => {
    render(<CancelBookingModal {...defaultProps} isLoading />);
    expect(screen.getByRole('button', { name: /cancelar$/i })).toBeDisabled();
    expect(screen.getByRole('button', { name: /confirmar cancelaci.n/i })).toBeDisabled();
  });

  it('clears the reason typed for a previous booking when reopened for a new one', async () => {
    const user = userEvent.setup();
    const { rerender } = render(<CancelBookingModal {...defaultProps} booking={bookingA} />);

    await user.type(screen.getByPlaceholderText(/motivo de la cancelaci.n/i), 'Lluvia');
    rerender(<CancelBookingModal {...defaultProps} booking={bookingA} open={false} />);
    rerender(<CancelBookingModal {...defaultProps} booking={bookingB} open />);

    expect(screen.getByPlaceholderText(/motivo de la cancelaci.n/i)).toHaveValue('');
  });
});
