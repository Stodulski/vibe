import { RouterProvider, createMemoryRouter, Link } from 'react-router-dom';
import { useState } from 'react';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { UnsavedChangesDialog } from '@/shared/components/common/UnsavedChangesDialog';
import { useUnsavedChangesBlocker } from './useUnsavedChangesBlocker';

function DirtyForm() {
  const [value, setValue] = useState('');
  const blocker = useUnsavedChangesBlocker(value !== '');

  return (
    <div>
      <label htmlFor="name">Nombre</label>
      <input
        id="name"
        value={value}
        onChange={(e) => {
          setValue(e.target.value);
        }}
      />
      <Link to="/dashboard">Ir al dashboard</Link>
      <Link to="/form?tab=horarios">Otra pestaña</Link>
      <UnsavedChangesDialog blocker={blocker} />
    </div>
  );
}

function renderAt() {
  const router = createMemoryRouter(
    [
      { path: '/form', element: <DirtyForm /> },
      { path: '/dashboard', element: <p>Dashboard</p> },
    ],
    { initialEntries: ['/form'] },
  );
  render(<RouterProvider router={router} />);
}

// FORM-11: nothing in the app used to intercept a navigation, so a mis-clicked
// sidebar link unmounted a half-filled form and took the work with it.
describe('useUnsavedChangesBlocker', () => {
  it('lets a clean form navigate away with no questions asked', async () => {
    const user = userEvent.setup();
    renderAt();

    await user.click(screen.getByRole('link', { name: 'Ir al dashboard' }));

    expect(await screen.findByText('Dashboard')).toBeInTheDocument();
  });

  it('asks before leaving a form with unsaved changes, and staying keeps what was typed', async () => {
    const user = userEvent.setup();
    renderAt();

    await user.type(screen.getByLabelText('Nombre'), 'Club Norte');
    await user.click(screen.getByRole('link', { name: 'Ir al dashboard' }));

    expect(await screen.findByText('Tenés cambios sin guardar')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Cancelar' }));

    await waitFor(() => {
      expect(screen.queryByText('Tenés cambios sin guardar')).not.toBeInTheDocument();
    });
    expect(screen.getByLabelText('Nombre')).toHaveValue('Club Norte');
    expect(screen.queryByText('Dashboard')).not.toBeInTheDocument();
  });

  it('lets the navigation through once leaving is confirmed', async () => {
    const user = userEvent.setup();
    renderAt();

    await user.type(screen.getByLabelText('Nombre'), 'Club Norte');
    await user.click(screen.getByRole('link', { name: 'Ir al dashboard' }));
    await user.click(await screen.findByRole('button', { name: 'Salir sin guardar' }));

    expect(await screen.findByText('Dashboard')).toBeInTheDocument();
  });

  // A `?tab=` on the same page is not leaving the form, and a confirmation
  // there would be a dialog about nothing.
  it('does not block a query-string change on the same route', async () => {
    const user = userEvent.setup();
    renderAt();

    await user.type(screen.getByLabelText('Nombre'), 'Club Norte');
    await user.click(screen.getByRole('link', { name: 'Otra pestaña' }));

    expect(screen.queryByText('Tenés cambios sin guardar')).not.toBeInTheDocument();
  });
});
