import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ES_AR } from '@/shared/i18n/es_AR';
import { AvailabilitySection } from './AvailabilitySection';

const t = ES_AR;

vi.mock('@/features/public-booking/components/CourtSelector', () => ({
  CourtSelector: () => <div data-testid="court-selector">CourtSelector</div>,
}));
vi.mock('@/features/public-booking/components/SkeletonSlotGrid', () => ({
  SkeletonSlotGrid: () => <div data-testid="skeleton-grid">Loading</div>,
}));

const noop = () => {
  /* no-op */
};

function renderSection(overrides: Partial<Parameters<typeof AvailabilitySection>[0]> = {}) {
  return render(
    <AvailabilitySection
      isLoading={false}
      isError={false}
      onRetry={noop}
      isStale={false}
      isOpen={true}
      dateStr="2026-03-20"
      courts={[]}
      selectedSlot={null}
      onSelect={noop}
      onContinue={noop}
      pendingStartTime={null}
      onPendingStartTimeChange={noop}
      {...overrides}
    />,
  );
}

describe('AvailabilitySection', () => {
  it('renders the error state with a retry button when isError is true, not "closed"', () => {
    renderSection({ isError: true, isOpen: false });
    expect(screen.getByText(t.publicBooking.availabilityLoadError)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: t.publicBooking.tryAgain })).toBeInTheDocument();
    expect(screen.queryByText(t.publicBooking.closed)).not.toBeInTheDocument();
  });

  it('calls onRetry when the retry button is clicked', async () => {
    const onRetry = vi.fn();
    const user = userEvent.setup();
    renderSection({ isError: true, isOpen: false, onRetry });
    await user.click(screen.getByRole('button', { name: t.publicBooking.tryAgain }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });

  it('renders "closed" only when isOpen is false and there is no error', () => {
    renderSection({ isError: false, isOpen: false });
    expect(screen.getByText(t.publicBooking.closed)).toBeInTheDocument();
  });

  it('renders the court grid when isOpen is true, even with a prior error cleared', () => {
    renderSection({ isError: false, isOpen: true });
    expect(screen.getByTestId('court-selector')).toBeInTheDocument();
    expect(screen.queryByText(t.publicBooking.closed)).not.toBeInTheDocument();
  });

  it('renders the loading skeleton before evaluating error or open state', () => {
    renderSection({ isLoading: true, isError: true, isOpen: false });
    expect(screen.getByTestId('skeleton-grid')).toBeInTheDocument();
    expect(screen.queryByText(t.publicBooking.availabilityLoadError)).not.toBeInTheDocument();
  });
});
