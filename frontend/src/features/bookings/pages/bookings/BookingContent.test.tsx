import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BookingContent } from './BookingContent';
import { makeBooking } from '@/test/factories';

vi.mock('@/shared/components/ui/skeleton', () => ({
  Skeleton: ({ className }: { className?: string }) => <div data-testid="skeleton" className={className} />,
}));
vi.mock('@/shared/components/common/EmptyState', () => ({
  EmptyState: ({ title, actionLabel, onAction }: { title: string; actionLabel?: string; onAction?: () => void }) => (
    <div data-testid="empty-state">
      {title}
      {actionLabel && onAction && <button onClick={onAction}>{actionLabel}</button>}
    </div>
  ),
}));
// Renders `emptyState` because that is now the calendar's job: an empty day
// still shows its court timeline, so the message is handed down rather than
// short-circuiting above it.
vi.mock('@/features/bookings/components/BookingCalendar', () => ({
  BookingCalendar: ({ emptyState }: { emptyState?: React.ReactNode }) => (
    <div data-testid="booking-calendar">{emptyState ?? 'Calendar'}</div>
  ),
}));

const baseProps = {
  isLoading: false,
  isError: false,
  onRetry: vi.fn(),
  filteredBookings: [],
  blockedSlots: [],
  isPast: false,
  selectedDate: '2026-03-18',
  courts: [],
  schedules: [],
  onSelectBooking: vi.fn(),
  onCreateFromSlot: vi.fn(),
  onOpenCreate: vi.fn(),
};

describe('BookingContent', () => {
  it('shows loading skeletons when isLoading', () => {
    render(<BookingContent {...baseProps} isLoading={true} />);
    expect(screen.getAllByTestId('skeleton').length).toBeGreaterThan(0);
  });

  it('shows empty state when no bookings and no filters', () => {
    render(<BookingContent {...baseProps} />);
    expect(screen.getByTestId('empty-state')).toHaveTextContent('No hay reservas para esta fecha');
  });

  it('renders calendar view', () => {
    render(<BookingContent {...baseProps} filteredBookings={[makeBooking({ id: '1' })]} />);
    expect(screen.getByTestId('booking-calendar')).toBeInTheDocument();
  });

  it('shows the error state with a retry action instead of the empty state when the query fails', () => {
    render(<BookingContent {...baseProps} isError={true} />);
    expect(screen.getByTestId('empty-state')).toHaveTextContent('No pudimos cargar los datos.');
    expect(screen.getByRole('button', { name: 'Reintentar' })).toBeInTheDocument();
    expect(screen.queryByText('No hay reservas para esta fecha')).not.toBeInTheDocument();
    expect(screen.queryByTestId('booking-calendar')).not.toBeInTheDocument();
  });

  it('calls onRetry when the retry button is clicked', async () => {
    const onRetry = vi.fn();
    render(<BookingContent {...baseProps} isError={true} onRetry={onRetry} />);
    await userEvent.click(screen.getByRole('button', { name: 'Reintentar' }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });
});
