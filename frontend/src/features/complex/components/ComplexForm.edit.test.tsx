import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderWithProviders, screen, userEvent, waitFor } from '@/test/test-utils';
import { mockUpdateMutate } from './complex-form-test-mocks';
import { ComplexForm } from './ComplexForm';

// latitude/longitude null here on purpose: complexes created outside this exact
// form (e2e/API fixtures today, any non-form path in general) never get resolved
// coordinates, and editing one of those must not be blocked by it.
const complexWithoutCoordinates = {
  id: 'c1',
  owner_id: 'u1',
  name: 'Club Test',
  slug: 'club-test',
  amenities: [],
  payments_enabled: false,
  address: 'Av. Test 123',
  city: 'Buenos Aires',
  province: 'CABA',
  country_code: 'AR',
  currency: 'ARS',
  phone: '1155550000',
  email: null,
  logo_url: null,
  cover_url: null,
  deposit_percentage: 30,
  cancellation_hours: 24,
  latitude: null,
  longitude: null,
  is_active: true,
  created_at: '2026-01-10T00:00:00Z',
  updated_at: '2026-01-10T00:00:00Z',
  version: 1,
};

describe('ComplexForm edit mode and interactions', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders update button for edit mode', () => {
    renderWithProviders(<ComplexForm complex={complexWithoutCoordinates} />);
    expect(screen.getByRole('button', { name: /guardar cambios/i })).toBeInTheDocument();
  });

  it('derives the public URL field from the name as it is typed', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ComplexForm />);

    await user.type(screen.getByLabelText(/nombre/i), 'Mi Club Padel');

    expect(screen.getByLabelText(/url pública/i)).toHaveValue('mi-club-padel');
  });

  it('lets an existing complex change its public URL and warns while it differs', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ComplexForm complex={complexWithoutCoordinates} />);

    const slugInput = screen.getByLabelText(/url pública/i);
    expect(slugInput).not.toHaveAttribute('readonly');
    // Untouched: still the complex's own slug, so no warning yet.
    expect(screen.queryByText(/dejan de funcionar/i)).not.toBeInTheDocument();

    await user.clear(slugInput);
    await user.type(slugInput, 'club-nuevo');

    expect(screen.getByText(/dejan de funcionar/i)).toBeInTheDocument();
  });

  it('saves an unrelated field change without re-selecting the address, for a complex with no coordinates', async () => {
    const user = userEvent.setup();
    renderWithProviders(<ComplexForm complex={complexWithoutCoordinates} />);

    await user.type(screen.getByLabelText(/^teléfono/i), '99');
    await user.click(screen.getByRole('button', { name: /guardar cambios/i }));

    await waitFor(() => {
      expect(mockUpdateMutate).toHaveBeenCalledTimes(1);
    });
    expect(screen.queryByText(/seleccion.*direcci.n de la lista/i)).not.toBeInTheDocument();
  });
});
