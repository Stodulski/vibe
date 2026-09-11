import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { makeConsumedHttpError } from '@/test/factories';
import { ES_AR } from '@/shared/i18n/es_AR';
import { useComplexBySlug } from '@/features/public-booking';

const t = ES_AR;

vi.mock('@/shared/hooks/usePageTitle', () => ({
  usePageTitle: vi.fn(),
  useOGTags: vi.fn(),
  useCanonical: vi.fn(),
  useStructuredData: vi.fn(),
}));
vi.mock('@/features/public-booking/hooks/useComplexBySlug', () => ({
  useComplexBySlug: vi.fn().mockReturnValue({ data: undefined, isLoading: true, isError: false }),
}));
vi.mock('@/features/public-booking/hooks/useAvailability', () => ({
  useAvailability: vi.fn().mockReturnValue({ data: undefined, isLoading: true }),
}));
vi.mock('@/features/public-booking/components/ComplexHeader', () => ({
  ComplexHeader: () => <div data-testid="complex-header">Header</div>,
}));
vi.mock('@/features/public-booking/components/DateSelector', () => ({
  DateSelector: () => <div data-testid="date-selector">DateSelector</div>,
}));
vi.mock('@/features/public-booking/components/CourtSelector', () => ({
  CourtSelector: () => <div data-testid="court-selector">CourtSelector</div>,
}));
vi.mock('@/features/public-booking/components/StepIndicator', () => ({
  StepIndicator: () => <div data-testid="step-indicator">Steps</div>,
}));
vi.mock('@/features/public-booking/components/SkeletonComplexHeader', () => ({
  SkeletonComplexHeader: () => <div data-testid="skeleton-header">Loading</div>,
}));
vi.mock('@/features/public-booking/components/SkeletonSlotGrid', () => ({
  SkeletonSlotGrid: () => <div data-testid="skeleton-grid">Loading</div>,
}));

describe('ComplexPage', () => {
  it('renders without crashing', async () => {
    const Page = (await import('./ComplexPage')).default;
    const { container } = render(
      <MemoryRouter initialEntries={['/test-club']}>
        <Page />
      </MemoryRouter>,
    );
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
    render(
      <MemoryRouter initialEntries={['/test-club']}>
        <Page />
      </MemoryRouter>,
    );
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
    render(
      <MemoryRouter initialEntries={['/test-club']}>
        <Page />
      </MemoryRouter>,
    );

    expect(await screen.findByText(t.publicBooking.complexLoadError)).toBeInTheDocument();
    expect(screen.queryByText(t.publicBooking.complexNotFound)).not.toBeInTheDocument();

    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: t.publicBooking.tryAgain }));
    expect(refetch).toHaveBeenCalledTimes(1);
  });
});
