import { render, screen } from '@testing-library/react';
import { NoBookingsEmptyState } from './BookingEmptyStates';

describe('NoBookingsEmptyState', () => {
  it('renders its "create" action as outline, since the Reservas header already has a primary "Nueva reserva" button', () => {
    render(<NoBookingsEmptyState isPast={false} onOpenCreate={vi.fn()} />);
    const button = screen.getByRole('button', { name: /nueva reserva/i });
    expect(button).toHaveAttribute('data-variant', 'outline');
  });
});
