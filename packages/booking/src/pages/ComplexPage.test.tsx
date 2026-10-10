import { describe, it, expect, vi } from 'vitest';
import { screen } from '@testing-library/react';
import { renderBooking } from '../test/booking';
import userEvent from '@testing-library/user-event';
import { makeConsumedHttpError } from '../test/factories';
import { BOOKING_MESSAGES } from '../messages';
import { useComplexBySlug } from '../hooks/useComplexBySlug';

const t = BOOKING_MESSAGES;

vi.mock('../hooks/useComplexBySlug', () => ({
  useComplexBySlug: vi.fn().mockReturnValue({ data: undefined, isLoading: true, isError: false }),
}));
vi.mock('../hooks/useAvailability', () => ({
  useAvailability: vi.fn().mockReturnValue({ data: undefined, isLoading: true }),
}));
vi.mock('../components/ComplexHeader', () => ({
  ComplexHeader: () => <div data-testid="complex-header">Header</div>,
}));
vi.mock('../components/DateSelector', () => ({
  DateSelector: () => <div data-testid="date-selector">DateSelector</div>,
}));
vi.mock('../components/CourtSelector', () => ({
  CourtSelector: () => <div data-testid="court-selector">CourtSelector</div>,
}));
vi.mock('../components/StepIndicator', () => ({
  StepIndicator: () => <div data-testid="step-indicator">Steps</div>,
}));
vi.mock('../components/SkeletonComplexHeader', () => ({
  SkeletonComplexHeader: () => <div data-testid="skeleton-header">Loading</div>,
}));
vi.mock('../components/SkeletonSlotGrid', () => ({
  SkeletonSlotGrid: () => <div data-testid="skeleton-grid">Loading</div>,
}));

describe('ComplexPage', () => {
  it('renders without crashing', async () => {
    const Page = (await import('./ComplexPage')).default;
    const { container } = renderBooking(<Page slug="test-club" />);
    expect(container).toBeDefined();
  });
});

describe('ComplexPage error vs not-found (useComplexBySlug)', () => {
  it('shows the not-found copy on a 404', async () => {
    vi.mocked(useComplexBySlug).mockReturnValueOnce({
      data: undefined,
      isLoading: false,
      error: await makeConsumedHttpError(404, {}),
      refetch: vi.fn(),
    } as unknown as ReturnType<typeof useComplexBySlug>);
    const Page = (await import('./ComplexPage')).default;
    renderBooking(<Page slug="test-club" />);
    expect(await screen.findByText(t.publicBooking.complexNotFound)).toBeInTheDocument();
  });

  it('shows a retryable error, not the not-found copy, on a 500', async () => {
    const refetch = vi.fn();
    vi.mocked(useComplexBySlug).mockReturnValueOnce({
      data: undefined,
      isLoading: false,
      error: await makeConsumedHttpError(500, {}),
      refetch,
    } as unknown as ReturnType<typeof useComplexBySlug>);
    const Page = (await import('./ComplexPage')).default;
    renderBooking(<Page slug="test-club" />);

    expect(await screen.findByText(t.publicBooking.complexLoadError)).toBeInTheDocument();
    expect(screen.queryByText(t.publicBooking.complexNotFound)).not.toBeInTheDocument();

    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: t.publicBooking.tryAgain }));
    expect(refetch).toHaveBeenCalledTimes(1);
  });
});
