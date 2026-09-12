import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { DesktopComplexesTable } from './DesktopComplexesTable';
import type { AdminComplexRow } from '@/shared/types/api.types';

const mockComplex: AdminComplexRow = {
  id: 'c1',
  owner_id: 'u1',
  owner_name: 'Juan Perez',
  owner_email: 'juan@test.com',
  name: 'Padel Club Norte',
  slug: 'padel-club-norte',
  city: 'Buenos Aires',
  is_active: true,
  courts_count: 4,
  mp_connected: true,
  created_at: '2026-01-10T10:00:00Z',
};

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location">{location.pathname}</div>;
}

function renderTable(complexes: AdminComplexRow[] = [mockComplex]) {
  return render(
    <MemoryRouter initialEntries={['/admin/complexes']}>
      <LocationProbe />
      <DesktopComplexesTable complexes={complexes} />
    </MemoryRouter>,
  );
}

// Finding M12: the row used to be `<TableRow role="button" tabIndex={0}>`,
// which strips the row/cell semantics and gives the row a run-on accessible
// name made of every column's text. The row's primary action is now a real
// link inside the first cell.
describe('DesktopComplexesTable — accessible row navigation', () => {
  it('keeps the row a plain row and exposes the complex name as a real link', () => {
    renderTable();

    const row = screen.getByRole('row', { name: /padel club norte/i });
    expect(row).not.toHaveAttribute('role', 'button');
    expect(row).not.toHaveAttribute('tabindex');

    const link = screen.getByRole('link', { name: 'Padel Club Norte' });
    expect(link).toHaveAttribute('href', '/admin/complexes/c1');
  });

  it('navigates with the keyboard alone (Tab then Enter)', async () => {
    const user = userEvent.setup();
    renderTable();

    await user.tab();
    expect(screen.getByRole('link', { name: 'Padel Club Norte' })).toHaveFocus();

    await user.keyboard('{Enter}');
    expect(await screen.findByTestId('location')).toHaveTextContent('/admin/complexes/c1');
  });
});
