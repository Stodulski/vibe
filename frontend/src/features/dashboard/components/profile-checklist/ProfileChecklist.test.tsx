import { describe, it, expect, vi, beforeEach } from 'vitest';
import { screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
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

const EXPANDED_KEY = 'vibe.profileChecklist.expanded';
const WHY_PHOTOS = 'Las fotos hacen que más jugadores elijan tu complejo.';

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

/** Cover, description, hours and priced court open; logo, map, services and phone done. */
function fourOpen() {
  mockQueries({ schedules: makeDefaultWeekSchedules(), courts: [] });
  return makeCompleteComplex({ cover_url: null, description: null });
}

beforeEach(() => {
  vi.clearAllMocks();
  window.localStorage.clear();
});

describe('ProfileChecklist: collapsed', () => {
  it('renders nothing while the schedules or courts are still loading', () => {
    mockQueries({ schedules: undefined, courts: undefined });

    const { container } = renderChecklist(makeCompleteComplex());

    expect(container).toBeEmptyDOMElement();
  });

  it('collapsed, shows the progress and only the next pending item', () => {
    renderChecklist(fourOpen());

    expect(screen.getByRole('heading', { name: 'Completá tu perfil' })).toBeInTheDocument();
    expect(screen.getByText('4 de 8 listos')).toBeInTheDocument();
    expect(screen.getByText(WHY_PHOTOS)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Subí la foto de portada' })).toHaveAttribute('href', '/settings');
    expect(screen.queryByRole('link', { name: 'Escribí tu descripción' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Editá los horarios' })).not.toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Cargá los precios' })).not.toBeInTheDocument();
  });

  it('the disclosure reports the pending count and starts collapsed', () => {
    renderChecklist(fourOpen());

    const toggle = screen.getByRole('button', { name: 'Ver los 4 pendientes' });
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
  });

  it('skips completed items when choosing the next one', () => {
    mockQueries({ schedules: reviewedHours(), courts: [makeActivePricedCourt()] });

    renderChecklist(makeCompleteComplex({ cover_url: 'https://cdn.example.com/c.jpg', description: null }));

    expect(screen.getByRole('link', { name: 'Escribí tu descripción' })).toBeInTheDocument();
    expect(screen.queryByRole('link', { name: 'Subí la foto de portada' })).not.toBeInTheDocument();
  });
});

describe('ProfileChecklist: expanded', () => {
  it('expanding lists every pending item once, with a chevron link to its destination', async () => {
    const user = userEvent.setup();
    renderChecklist(fourOpen());

    await user.click(screen.getByRole('button', { name: 'Ver los 4 pendientes' }));

    expect(screen.getByRole('button', { name: 'Ocultar pendientes' })).toHaveAttribute('aria-expanded', 'true');
    const list = screen.getByRole('list');
    const rows = within(list).getAllByRole('listitem');
    expect(rows).toHaveLength(4);
    expect(within(list).getByText('Foto de portada')).toBeInTheDocument();
    expect(within(list).getByText('Descripción')).toBeInTheDocument();
    expect(within(list).getByText('Revisá tus horarios')).toBeInTheDocument();
    expect(within(list).getByText('Cancha con precio')).toBeInTheDocument();
    expect(within(list).queryByText('Logo')).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Subí la foto de portada' })).toHaveAttribute('href', '/settings');
    expect(screen.getByRole('link', { name: 'Escribí tu descripción' })).toHaveAttribute('href', '/settings');
    expect(screen.getByRole('link', { name: 'Editá los horarios' })).toHaveAttribute('href', '/settings?tab=schedules');
    expect(screen.getByRole('link', { name: 'Cargá los precios' })).toHaveAttribute('href', '/courts');
  });

  it('expanded rows carry no per-item reason text', async () => {
    const user = userEvent.setup();
    renderChecklist(fourOpen());

    await user.click(screen.getByRole('button', { name: 'Ver los 4 pendientes' }));

    expect(screen.queryByText(WHY_PHOTOS)).not.toBeInTheDocument();
  });

  it('collapsing again hides the list and brings back the next item', async () => {
    const user = userEvent.setup();
    renderChecklist(fourOpen());
    await user.click(screen.getByRole('button', { name: 'Ver los 4 pendientes' }));

    await user.click(screen.getByRole('button', { name: 'Ocultar pendientes' }));

    expect(screen.queryByRole('list')).not.toBeInTheDocument();
    expect(screen.getByText(WHY_PHOTOS)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Ver los 4 pendientes' })).toHaveAttribute('aria-expanded', 'false');
  });

  it('shows no disclosure when only one item is pending, because the next item already is that item', () => {
    mockQueries({ schedules: reviewedHours(), courts: [makeActivePricedCourt()] });

    renderChecklist(makeCompleteComplex({ description: null }));

    expect(screen.getByRole('link', { name: 'Escribí tu descripción' })).toBeInTheDocument();
    expect(screen.queryByRole('button', { name: /pendientes/ })).not.toBeInTheDocument();
  });
});

describe('ProfileChecklist: state', () => {
  it('remembers an expanded choice per browser', async () => {
    const user = userEvent.setup();
    window.localStorage.setItem(EXPANDED_KEY, '1');
    renderChecklist(fourOpen());

    expect(screen.getByRole('button', { name: 'Ocultar pendientes' })).toHaveAttribute('aria-expanded', 'true');

    await user.click(screen.getByRole('button', { name: 'Ocultar pendientes' }));

    expect(window.localStorage.getItem(EXPANDED_KEY)).toBe('0');
  });

  it('exposes progress as a progressbar with the completed count', () => {
    renderChecklist(fourOpen());

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
