import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { OwnedComplexesList } from './OwnedComplexesList';
import type { Complex } from '@/shared/types/api.types';

const mockComplex: Complex = {
  id: 'c1',
  owner_id: 'u1',
  name: 'Padel Club Norte',
  slug: 'padel-club-norte',
  amenities: [],
  payments_enabled: false,
  address: 'Av. Libertador 1234',
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
  latitude: -34.5,
  longitude: -58.5,
  is_active: true,
  created_at: '2026-01-10T10:00:00Z',
  updated_at: '2026-01-10T10:00:00Z',
  version: 1,
};

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location">{location.pathname}</div>;
}

function renderList(complexes: Complex[] = [mockComplex]) {
  return render(
    <MemoryRouter initialEntries={['/admin/users/u1']}>
      <LocationProbe />
      <OwnedComplexesList complexes={complexes} />
    </MemoryRouter>,
  );
}

// Finding M12: each card was a `Panel` (a plain `div`) with an `onClick`,
// so it never got focus and did not respond to Enter. It is now a real link.
describe('OwnedComplexesList — accessible card navigation', () => {
  it('exposes each complex card as a real link', () => {
    renderList();

    const link = screen.getByRole('link', { name: /padel club norte/i });
    expect(link).toHaveAttribute('href', '/admin/complexes/c1');
  });

  it('navigates with the keyboard alone (Tab then Enter)', async () => {
    const user = userEvent.setup();
    renderList();

    await user.tab();
    expect(screen.getByRole('link', { name: /padel club norte/i })).toHaveFocus();

    await user.keyboard('{Enter}');
    expect(await screen.findByTestId('location')).toHaveTextContent('/admin/complexes/c1');
  });
});
