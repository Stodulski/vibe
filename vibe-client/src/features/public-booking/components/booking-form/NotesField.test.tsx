import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useForm } from 'react-hook-form';
import { ES_AR } from '@/shared/i18n/es_AR';
import type { PublicBookingFormData } from '../../schemas/public-booking.schemas';
import { NotesField } from './NotesField';

const t = ES_AR;

function Wrapper() {
  const { register } = useForm<PublicBookingFormData>();
  return <NotesField register={register} />;
}

describe('NotesField', () => {
  it('associates the textarea with a label once opened, not just a placeholder', async () => {
    const user = userEvent.setup();
    render(<Wrapper />);
    await user.click(screen.getByRole('button', { name: new RegExp(t.publicBooking.notes, 'i') }));

    // `getByLabelText` only succeeds when the textarea has a real associated
    // label (a placeholder does not count) — this is what fails without the fix.
    expect(screen.getByLabelText(t.publicBooking.notes)).toBeInTheDocument();
  });

  it('points the toggle button at the textarea it controls via aria-controls', async () => {
    const user = userEvent.setup();
    render(<Wrapper />);
    const toggle = screen.getByRole('button', { name: new RegExp(t.publicBooking.notes, 'i') });
    await user.click(toggle);

    const textarea = screen.getByLabelText(t.publicBooking.notes);
    expect(toggle).toHaveAttribute('aria-controls', textarea.id);
  });
});
