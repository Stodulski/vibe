import { makePrice, makeComplex } from '@/test/factories';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent, within } from '@/test/test-utils';
import { CourtGrid } from './CourtGrid';
import type { CourtWithPrices } from '@/shared/types/api.types';

vi.mock('../hooks/useUpdateCourt', () => ({
  useUpdateCourt: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
}));

vi.mock('../hooks/useDeleteCourt', () => ({
  useDeleteCourt: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
}));

vi.mock('../hooks/useUpdatePrices', () => ({
  useUpdatePrices: () => ({
    mutate: vi.fn(),
    isPending: false,
  }),
}));

// PriceConfig's form (usePriceConfigForm) also reads the complex and its
// schedules; stubbed out since this file's tests don't exercise pricing.
vi.mock('@/features/complex', () => ({
  useComplex: () => ({ data: makeComplex({ id: 'c1', slug: 'los-alamos' }) }),
  useSchedules: () => ({ data: [] }),
}));

const mockCourts: CourtWithPrices[] = [
  {
    id: 'ct1',
    complex_id: 'c1',
    name: 'Cancha 1',
    sport: 'padel',
    court_type: 'outdoor',
    is_active: true,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    prices: [
      makePrice({
        id: 'p1',
        court_id: 'ct1',
        price: 1500000,
        day_type: 'monday',
        time_from: '08:00',
        time_to: '23:00',
      }),
    ],
  },
  {
    id: 'ct2',
    complex_id: 'c1',
    name: 'Cancha 2',
    sport: 'tennis',
    court_type: 'indoor',
    is_active: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    prices: [],
  },
];

describe('CourtGrid', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders all court names', () => {
    renderWithProviders(<CourtGrid courts={mockCourts} complexId="c1" />);
    expect(screen.getByText('Cancha 1')).toBeInTheDocument();
    expect(screen.getByText('Cancha 2')).toBeInTheDocument();
  });

  it('renders court sport types', () => {
    renderWithProviders(<CourtGrid courts={mockCourts} complexId="c1" />);
    expect(screen.getAllByText(/p.del/i).length).toBeGreaterThan(0);
    expect(screen.getAllByText(/tenis/i).length).toBeGreaterThan(0);
  });

  it('renders edit buttons for each court', () => {
    renderWithProviders(<CourtGrid courts={mockCourts} complexId="c1" />);
    const editButtons = screen.getAllByRole('button', { name: /editar/i });
    expect(editButtons).toHaveLength(2);
  });

  it('renders price buttons for each court', () => {
    renderWithProviders(<CourtGrid courts={mockCourts} complexId="c1" />);
    const priceButtons = screen.getAllByRole('button', { name: /precios/i });
    expect(priceButtons).toHaveLength(2);
  });

  // Was anchored on `[data-tour="courts-grid"]`, which went with the tour.
  // Asserting on the rendered cards instead of on the container that holds
  // them says the same thing without depending on a hook that exists for
  // something else — and there is no hook left to depend on.
  it('renders no cards when there are no courts', () => {
    renderWithProviders(<CourtGrid courts={[]} complexId="c1" />);
    expect(screen.queryAllByRole('article')).toHaveLength(0);
    expect(screen.queryByText(/cancha 1/i)).not.toBeInTheDocument();
  });

  // CourtGrid used to keep the clicked court itself in state, so a `courts`
  // update while the price dialog stayed open (a refetch, another tab, the
  // optimistic update in useUpdateCourt) never reached it — the dialog kept
  // showing whatever was true at the moment "Precios" was clicked. Storing
  // only the id and looking the court up from the live `courts` prop on every
  // render means an update in place is reflected immediately.
  it('reflects a court update while its price dialog stays open', async () => {
    const user = userEvent.setup();
    const { rerender } = renderWithProviders(<CourtGrid courts={mockCourts} complexId="c1" />);

    const firstPriceButton = screen.getAllByRole('button', { name: /precios/i })[0];
    if (!firstPriceButton) throw new Error('expected a "Precios" button');
    await user.click(firstPriceButton);
    const dialog = await screen.findByRole('dialog');
    expect(within(dialog).getByText(/Cancha 1/)).toBeInTheDocument();

    const renamedCourts = mockCourts.map((c) => (c.id === 'ct1' ? { ...c, name: 'Cancha Renombrada' } : c));
    rerender(<CourtGrid courts={renamedCourts} complexId="c1" />);

    expect(await within(dialog).findByText(/Cancha Renombrada/)).toBeInTheDocument();
  });
});
