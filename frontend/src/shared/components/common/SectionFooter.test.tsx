import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { SectionFooter, SectionFooterCancel, SectionFooterSubmit } from './SectionFooter';

describe('SectionFooter', () => {
  describe('flag-props shape (legacy callers)', () => {
    it('renders cancel and submit buttons and forwards clicks', async () => {
      const user = userEvent.setup();
      const onCancel = vi.fn();
      const onSubmit = vi.fn();
      render(<SectionFooter onCancel={onCancel} onSubmit={onSubmit} cancelLabel="Cancelar" submitLabel="Guardar" />);

      await user.click(screen.getByRole('button', { name: 'Cancelar' }));
      expect(onCancel).toHaveBeenCalledTimes(1);

      await user.click(screen.getByRole('button', { name: 'Guardar' }));
      expect(onSubmit).toHaveBeenCalledTimes(1);
    });

    it('shows a spinner and disables the submit button while pending', () => {
      render(<SectionFooter submitLabel="Guardar" pending />);
      const submitButton = screen.getByRole('button');
      expect(submitButton).toBeDisabled();
      expect(screen.queryByText('Guardar')).not.toBeInTheDocument();
    });
  });

  describe('composition shape (SectionFooterCancel / SectionFooterSubmit)', () => {
    it('renders custom children instead of the flag-props buttons', async () => {
      const user = userEvent.setup();
      const onCancel = vi.fn();
      const onSubmit = vi.fn();
      render(
        <SectionFooter>
          <SectionFooterCancel onClick={onCancel}>Volver</SectionFooterCancel>
          <SectionFooterSubmit onClick={onSubmit}>Confirmar</SectionFooterSubmit>
        </SectionFooter>,
      );

      await user.click(screen.getByRole('button', { name: 'Volver' }));
      expect(onCancel).toHaveBeenCalledTimes(1);

      await user.click(screen.getByRole('button', { name: 'Confirmar' }));
      expect(onSubmit).toHaveBeenCalledTimes(1);
    });

    it('SectionFooterSubmit shows a spinner and disables itself when pending', () => {
      render(
        <SectionFooter>
          <SectionFooterSubmit pending>Confirmar</SectionFooterSubmit>
        </SectionFooter>,
      );

      const submitButton = screen.getByRole('button');
      expect(submitButton).toBeDisabled();
      expect(screen.queryByText('Confirmar')).not.toBeInTheDocument();
    });
  });
});
