import { describe, it, expect, vi } from 'vitest';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { CancelRegistration } from './CancelRegistration';

const copy = {
  action: 'Cancelar inscripción',
  confirmTitle: 'Cancelar inscripción',
  confirmRefund: 'Te devolvemos lo que pagaste.',
  confirmNoRefund: 'No se devuelve lo que pagaste.',
};

describe('CancelRegistration', () => {
  it('dismisses the confirmation with "Volver", never a second "Cancelar"', async () => {
    const user = userEvent.setup();
    render(
      <CancelRegistration
        withinWindow
        deadline="2099-12-04T19:00:00-03:00"
        cancel={{ mutate: vi.fn(), isPending: false }}
        copy={copy}
      />,
    );
    await user.click(screen.getByRole('button', { name: copy.action }));
    const dialog = await screen.findByRole('alertdialog');
    expect(within(dialog).getByRole('button', { name: 'Volver' })).toBeInTheDocument();
    expect(within(dialog).queryByRole('button', { name: 'Cancelar' })).not.toBeInTheDocument();
  });
});
