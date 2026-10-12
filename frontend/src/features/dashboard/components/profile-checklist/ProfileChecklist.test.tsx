import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen } from '@testing-library/react';
import { ProfileChecklist } from './ProfileChecklist';
import { renderWithProviders } from '@/test/test-utils';
import { useSchedules } from '@/features/complex';
import { useCourts } from '@/features/courts';
import { makeActivePricedCourt, makeCompleteComplex, makeDefaultWeekSchedules, makeSchedule } from '@/test/factories';
import type { Complex, CourtWithPrices, Schedule } from '@/shared/types/api.types';

// Mocked at the hook file: the feature barrels re-export these, so the component
// under test picks up the mock through the public import path.
vi.mock('@/features/complex/hooks/useSchedules', () => ({ useSchedules: vi.fn() }));
vi.mock('@/features/courts/hooks/useCourts', () => ({ useCourts: vi.fn() }));

function mockQueries({
  schedules,
  courts,
}: {
  schedules?: Schedule[] | undefined;
  courts?: CourtWithPrices[] | undefined;
}) {
  vi.mocked(useSchedules).mockReturnValue({ data: schedules } as unknown as ReturnType<typeof useSchedules>);
  vi.mocked(useCourts).mockReturnValue({ data: courts } as unknown as ReturnType<typeof useCourts>);
}

/** Hours changed on one day, so the schedule counts as reviewed. */
const reviewedHours = () =>
  makeDefaultWeekSchedules().map((s) => (s.day === 'wednesday' ? makeSchedule({ ...s, open_time: '09:00' }) : s));

function renderChecklist(complex: Complex) {
  return renderWithProviders(<ProfileChecklist complex={complex} />);
}

describe('ProfileChecklist', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders nothing while the schedules or courts are still loading', () => {
    mockQueries({ schedules: undefined, courts: undefined });

    const { container } = renderChecklist(makeCompleteComplex());

    expect(container).toBeEmptyDOMElement();
  });

  it('lists each missing item with its reason and a link to the screen that fixes it', () => {
    mockQueries({ schedules: makeDefaultWeekSchedules(), courts: [] });
    const incomplete = makeCompleteComplex({ cover_url: null, description: null });

    renderChecklist(incomplete);

    expect(screen.getByRole('heading', { name: 'Completá tu perfil público' })).toBeInTheDocument();
    expect(screen.getByText('4 de 8 listos')).toBeInTheDocument();
    expect(screen.getByText('Las fotos hacen que más jugadores elijan tu complejo.')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Subí la foto de portada' })).toHaveAttribute('href', '/settings');
    expect(screen.getByRole('link', { name: 'Editá los horarios' })).toHaveAttribute('href', '/settings?tab=schedules');
    expect(screen.getByRole('link', { name: 'Cargá los precios' })).toHaveAttribute('href', '/courts');
  });

  it('does not list items that are already done', () => {
    mockQueries({ schedules: reviewedHours(), courts: [makeActivePricedCourt()] });
    const incomplete = makeCompleteComplex({ logo_url: null });

    renderChecklist(incomplete);

    expect(screen.getByRole('link', { name: 'Subí tu logo' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Editá los horarios' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Cargá los precios' })).not.toBeInTheDocument();
  });

  it('exposes progress as a progressbar with the completed count', () => {
    mockQueries({ schedules: makeDefaultWeekSchedules(), courts: [] });

    renderChecklist(makeCompleteComplex({ cover_url: null, description: null }));

    const bar = screen.getByRole('progressbar');
    expect(bar).toHaveAttribute('aria-valuenow', '4');
    expect(bar).toHaveAttribute('aria-valuemax', '8');
  });

  it('renders nothing once every item is done', () => {
    mockQueries({ schedules: reviewedHours(), courts: [makeActivePricedCourt()] });

    const { container } = renderChecklist(makeCompleteComplex());

    expect(container).toBeEmptyDOMElement();
  });
});
