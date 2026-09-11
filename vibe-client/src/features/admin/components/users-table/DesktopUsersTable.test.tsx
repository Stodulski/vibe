import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, useLocation } from 'react-router-dom';
import { DesktopUsersTable } from './DesktopUsersTable';
import type { AdminUserRow } from '@/shared/types/api.types';

const mockUser: AdminUserRow = {
  id: 'u1',
  email: 'juan@test.com',
  first_name: 'Juan',
  last_name: 'Perez',
  phone: '1155550000',
  role: 'owner',
  is_active: true,
  email_verified: true,
  created_at: '2026-01-10T10:00:00Z',
  complex_count: 2,
};

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location">{location.pathname}</div>;
}

function renderTable(users: AdminUserRow[] = [mockUser]) {
  return render(
    <MemoryRouter initialEntries={['/admin/users']}>
      <LocationProbe />
      <DesktopUsersTable users={users} />
    </MemoryRouter>,
  );
}

// Finding M12: the row used to be `<TableRow role="button" tabIndex={0}>`,
// which strips the row/cell semantics and gives the row a run-on accessible
// name made of every column's text. The row's primary action is now a real
// link inside the first cell.
describe('DesktopUsersTable — accessible row navigation', () => {
  it('keeps the row a plain row and exposes the user name as a real link', () => {
    renderTable();

    const row = screen.getByRole('row', { name: /juan perez/i });
    expect(row).not.toHaveAttribute('role', 'button');
    expect(row).not.toHaveAttribute('tabindex');

    const link = screen.getByRole('link', { name: 'Juan Perez' });
    expect(link).toHaveAttribute('href', '/admin/users/u1');
  });

  it('navigates with the keyboard alone (Tab then Enter)', async () => {
    const user = userEvent.setup();
    renderTable();

    await user.tab();
    expect(screen.getByRole('link', { name: 'Juan Perez' })).toHaveFocus();

    await user.keyboard('{Enter}');
    expect(await screen.findByTestId('location')).toHaveTextContent('/admin/users/u1');
  });
});
