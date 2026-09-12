import { z } from 'zod';
import { zodResolver } from '@hookform/resolvers/zod';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useAppForm, submitHandler } from './form';

const schema = z.object({ email: z.email('Email inválido') });
type Values = z.infer<typeof schema>;

function TestForm({ mode }: { mode?: 'onChange' }) {
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useAppForm<Values>({
    resolver: zodResolver(schema),
    defaultValues: { email: '' },
    ...(mode ? { mode } : {}),
  });

  return (
    <form
      onSubmit={submitHandler(handleSubmit, () => {
        /* noop */
      })}
      noValidate
    >
      <label htmlFor="email">Email</label>
      <input id="email" {...register('email')} />
      {errors.email && <p role="alert">{errors.email.message}</p>}
      <button type="submit">Enviar</button>
    </form>
  );
}

// FORM-05: react-hook-form's default is `onSubmit`, so a mistyped field was
// only ever reported after the whole form had been filled in and sent.
describe('useAppForm', () => {
  it('reports a bad value when the field is left, not only on submit', async () => {
    const user = userEvent.setup();
    render(<TestForm />);

    await user.type(screen.getByLabelText('Email'), 'no-es-un-email');
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();

    await user.tab();

    await waitFor(() => {
      expect(screen.getByRole('alert')).toHaveTextContent('Email inválido');
    });
  });

  it('stays quiet while the field is still being typed', async () => {
    const user = userEvent.setup();
    render(<TestForm />);

    await user.type(screen.getByLabelText('Email'), 'jua');

    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  // The default is spread over, not enforced: the blocked-slot modal drives a
  // submit button off the form's own validity and needs per-keystroke checks.
  it('lets a form that needs onChange ask for it', async () => {
    const user = userEvent.setup();
    render(<TestForm mode="onChange" />);

    await user.type(screen.getByLabelText('Email'), 'jua');

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
    });
  });

  // FORM-09: an input mounted with `value === undefined` is uncontrolled until
  // the first keystroke flips it, which React warns about and which breaks a
  // later `reset()` back to the initial value.
  it('starts every declared field controlled, with no React warning', async () => {
    const warn = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    const user = userEvent.setup();
    render(<TestForm />);

    expect(screen.getByLabelText('Email')).toHaveValue('');
    await user.type(screen.getByLabelText('Email'), 'a');

    expect(warn).not.toHaveBeenCalled();
    warn.mockRestore();
  });
});
