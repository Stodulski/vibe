import { makeCourt, makePrice } from '@/test/factories';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent } from '@/test/test-utils';
import { CourtTable } from './CourtTable';
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

const courtWithRange: CourtWithPrices = {
  ...makeCourt({ id: 'ct1', name: 'Cancha 1', sport: 'padel', court_type: 'outdoor', description: 'Con luz' }),
  prices: [
    makePrice({ id: 'p1', court_id: 'ct1', price: 1350000, day_type: 'monday' }),
    makePrice({ id: 'p2', court_id: 'ct1', price: 2850000, day_type: 'saturday' }),
  ],
};

const courtNoPrices: CourtWithPrices = {
  ...makeCourt({ id: 'ct2', name: 'Cancha 2', sport: 'tennis', court_type: 'indoor', is_active: false }),
  prices: [],
};

function renderTable(courts: CourtWithPrices[], overrides: Partial<Parameters<typeof CourtTable>[0]> = {}) {
  const props = { courts, complexId: 'c1', onEdit: vi.fn(), onPrices: vi.fn(), ...overrides };
  renderWithProviders(<CourtTable {...props} />);
  return props;
}

describe('CourtTable', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders as a table with one row per court', () => {
    renderTable([courtWithRange, courtNoPrices]);
    expect(screen.getByRole('table')).toBeInTheDocument();
    expect(screen.getByText('Cancha 1')).toBeInTheDocument();
    expect(screen.getByText('Cancha 2')).toBeInTheDocument();
  });

  it('shows the sport under the court name', () => {
    renderTable([courtWithRange]);
    expect(screen.getByText(/p.del/i)).toBeInTheDocument();
  });

  it('shows the court type', () => {
    renderTable([courtWithRange]);
    expect(screen.getByText('Descubierta')).toBeInTheDocument();
  });

  it('shows one general min–max price range across every band', () => {
    renderTable([courtWithRange]);
    // 1350000/2850000 centavos -> $13.500 / $28.500.
    expect(screen.getByText('$13.500 - $28.500')).toBeInTheDocument();
  });

  it("shows the card's empty-price wording when a court has no prices", () => {
    renderTable([courtNoPrices]);
    expect(screen.getByText('Sin precios configurados')).toBeInTheDocument();
  });

  it('shows the description with the empty-state string when there is none', () => {
    renderTable([courtWithRange, courtNoPrices]);
    expect(screen.getByText('Con luz')).toBeInTheDocument();
    expect(screen.getByText('Sin descripción')).toBeInTheDocument();
  });

  it('renders the active toggle', () => {
    renderTable([courtWithRange]);
    const toggle = screen.getByRole('switch');
    expect(toggle).toBeChecked();
  });

  it("the ⋯ menu opens Editar, Precios and Eliminar for a court's row", async () => {
    const user = userEvent.setup();
    const props = renderTable([courtWithRange]);

    await user.click(screen.getByRole('button', { name: /más acciones: cancha 1/i }));
    expect(await screen.findByText('Editar')).toBeInTheDocument();
    expect(screen.getByText('Precios')).toBeInTheDocument();
    expect(screen.getByText('Eliminar')).toBeInTheDocument();

    await user.click(screen.getByText('Editar'));
    expect(props.onEdit).toHaveBeenCalledWith(expect.objectContaining({ id: 'ct1' }));
  });

  it('calls onPrices from the row menu', async () => {
    const user = userEvent.setup();
    const props = renderTable([courtWithRange]);

    await user.click(screen.getByRole('button', { name: /más acciones: cancha 1/i }));
    await user.click(await screen.findByText('Precios'));

    expect(props.onPrices).toHaveBeenCalledWith(expect.objectContaining({ id: 'ct1' }));
  });
});
